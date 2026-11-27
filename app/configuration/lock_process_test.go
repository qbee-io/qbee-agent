// Copyright 2026 qbee.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0


//go:build linux

package configuration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

// lockProcessDirEnv is set when the test binary runs as a separate process executing configuration
// (see TestLockProcessHelper). The lock file and the after-command's process group are kept in this directory.
const lockProcessDirEnv = "QBEE_TEST_LOCK_PROCESS_DIR"

// TestLockProcessHelper isn't a test. It executes configuration in a separate process for the test below,
// with an after-command which runs until it's killed.
func TestLockProcessHelper(t *testing.T) {
	dir := os.Getenv(lockProcessDirEnv)
	if dir == "" {
		t.Skip("only used as a helper process")
	}

	srv, config := newBlockingExecution(t)
	srv.WithLockFile(filepath.Join(dir, "config.lock"))

	// the after-command records its process group, so the test knows it's running and can clean it up
	config.BundleData.FileDistribution.FileSets[0].AfterCommand =
		"echo $$ > " + filepath.Join(dir, "pgid.tmp") + " && mv " + filepath.Join(dir, "pgid.tmp") + " " +
			filepath.Join(dir, "pgid") + "; sleep 600 &"

	_ = srv.Execute(t.Context(), config)
}

// lockProcess is a separate process executing configuration (see TestLockProcessHelper).
type lockProcess struct {
	cmd *exec.Cmd

	// pgid is the process group of the after-command
	pgid int

	// exited is closed when the process exited
	exited chan struct{}
}

// startLockProcess starts a process executing configuration with the lock file in dir, and returns once its
// after-command is running, so the process holds the lock.
func startLockProcess(t *testing.T, dir string) *lockProcess {
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockProcessHelper$", "-test.count=1")
	cmd.Env = append(os.Environ(), lockProcessDirEnv+"="+dir)

	assert.NoError(t, cmd.Start())

	process := &lockProcess{cmd: cmd, exited: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		close(process.exited)
	}()

	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-process.exited

		if process.pgid > 0 {
			_ = syscall.Kill(-process.pgid, syscall.SIGKILL)
		}
	})

	deadline := time.Now().Add(30 * time.Second)
	for {
		if data, err := os.ReadFile(filepath.Join(dir, "pgid")); err == nil {
			process.pgid, err = strconv.Atoi(strings.TrimSpace(string(data)))
			assert.NoError(t, err)

			return process
		}

		if time.Now().After(deadline) {
			t.Fatal("after-command of the configuration execution didn't start")
		}

		select {
		case <-process.exited:
			t.Fatal("configuration execution process exited before its after-command started")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestService_Execute_LockOfKilledProcess(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "config.lock")

	// the process executing configuration is killed during its after-command (e.g. it crashed)
	other := startLockProcess(t, dir)
	assert.NoError(t, other.cmd.Process.Kill())
	<-other.exited

	srv := New(nil, t.TempDir(), t.TempDir()).WithLockFile(lockPath)

	// the lock file left behind was refreshed recently, so it could belong to a running execution
	assert.NoError(t, srv.Execute(t.Context(), newWatchdogConfig()))
	assert.Equal(t, srv.connectivityWatchdogThreshold.Load(), int64(0))

	// once it wasn't refreshed for lockStaleTimeout, it's taken over
	modTime := time.Now().Add(-lockStaleTimeout - time.Minute)
	assert.NoError(t, os.Chtimes(lockPath, modTime, modTime))

	assert.NoError(t, srv.Execute(t.Context(), newWatchdogConfig()))
	assert.Equal(t, srv.connectivityWatchdogThreshold.Load(), int64(5))

	// the execution released its lock
	_, err := os.Stat(lockPath)
	assert.True(t, os.IsNotExist(err))
}
