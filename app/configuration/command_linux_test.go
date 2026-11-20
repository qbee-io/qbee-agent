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
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

func TestRunCommand_CancelKillsSubprocesses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// the background process keeps the command's output open after the shell is killed
	start := time.Now()
	_, err := RunCommand(ctx, "sleep 30 & wait")

	assert.True(t, err != nil)
	assert.True(t, time.Since(start) < 5*time.Second)
}

func TestRunCommand_CancelKillsSubprocessesAfterShellExits(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// the shell exits right away, while its background process keeps the output open
	start := time.Now()
	output, err := RunCommand(ctx, "(sleep 30; echo done) &")

	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.Equal(t, string(output), "")
	assert.True(t, time.Since(start) < 5*time.Second)
}

func TestRunCommand_CancelClosesOutputOfDetachedSubprocesses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// the detached process isn't in the command's process group, so it isn't killed and keeps the output open
	start := time.Now()
	_, err := RunCommand(ctx, "setsid sleep 10 &")

	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.True(t, time.Since(start) < 5*time.Second)
}

func TestRunCommand_BackgroundSubprocessWaitingForShell(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// the background process keeps the output open until the shell is gone
	start := time.Now()
	output, err := RunCommand(ctx, "(while kill -0 $$ 2>/dev/null; do sleep 0.1; done; echo shell gone) &")

	assert.NoError(t, err)
	assert.Equal(t, string(output), "shell gone")
	assert.True(t, time.Since(start) < 5*time.Second)
}

func TestRunCommand_WaitsForBackgroundSubprocesses(t *testing.T) {
	// the background process keeps the command's output open after the shell exits
	output, err := RunCommand(context.Background(), "(sleep 2; echo done) &")

	assert.NoError(t, err)
	assert.Equal(t, string(output), "done")
}

func TestRunCommand_ReturnsExitError(t *testing.T) {
	output, err := RunCommand(context.Background(), "echo failed; exit 3")

	var exitErr *exec.ExitError
	assert.True(t, errors.As(err, &exitErr))
	assert.Equal(t, exitErr.ExitCode(), 3)
	assert.Equal(t, string(output), "failed")
}

func TestRunCommand_CombinesStdoutAndStderr(t *testing.T) {
	output, err := RunCommand(context.Background(), "echo out1; echo err >&2; echo out2")

	assert.NoError(t, err)
	assert.Equal(t, string(output), "out1\nerr\nout2")
}

func TestRunCommand_KeepsTailOfLargeOutput(t *testing.T) {
	// about 2MB of output, twice the limit
	output, err := RunCommand(context.Background(), "seq 1 300000")

	assert.NoError(t, err)
	assert.True(t, len(output) <= commandOutputBytesLimit)
	assert.True(t, len(output) > commandOutputBytesLimit/2)
	assert.True(t, strings.HasSuffix(string(output), "\n299999\n300000"))
	assert.False(t, strings.HasPrefix(string(output), "1\n2\n"))
}

func TestRunCommand_AlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	marker := filepath.Join(t.TempDir(), "marker")
	_, err := RunCommand(ctx, "touch "+marker)

	assert.True(t, errors.Is(err, context.Canceled))

	_, statErr := os.Stat(marker)
	assert.True(t, os.IsNotExist(statErr))
}

func TestRunCommand_BackgroundSubprocessFinishingBeforeDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// the background process outlives the shell, but finishes well before the deadline
	output, err := RunCommand(ctx, "(sleep 0.2; echo late) &")

	assert.NoError(t, err)
	assert.Equal(t, string(output), "late")
}

func TestRunCommand_NoLeaks(t *testing.T) {
	openFiles := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		assert.NoError(t, err)
		return len(entries)
	}

	filesBefore := openFiles()
	goroutinesBefore := runtime.NumGoroutine()

	for range 20 {
		_, _ = RunCommand(context.Background(), "echo ok")
		_, _ = RunCommand(context.Background(), "exit 1")

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		_, _ = RunCommand(ctx, "sleep 30 &")
		cancel()
	}

	// goroutines may need a moment to exit after the last command returned
	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > goroutinesBefore && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	assert.Equal(t, openFiles(), filesBefore)
	assert.True(t, runtime.NumGoroutine() <= goroutinesBefore)
}
