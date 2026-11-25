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

package configuration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

// newLockTestService returns a service and the execution lock it holds, released when the test finishes.
func newLockTestService(t *testing.T) (*Service, *executionLock) {
	srv := New(nil, t.TempDir(), t.TempDir()).WithLockFile(filepath.Join(t.TempDir(), "config.lock"))

	lock, err := srv.acquireLock()
	assert.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(lock.path) })

	return srv, lock
}

func setLockAge(t *testing.T, lock *executionLock, age time.Duration) {
	modTime := time.Now().Add(-age)
	assert.NoError(t, os.Chtimes(lock.path, modTime, modTime))
}

func lockAcquired(srv *Service) bool {
	_, err := srv.acquireLock()
	return err == nil
}

func TestService_acquireLock(t *testing.T) {
	srv, lock := newLockTestService(t)

	// lock held by another run, which keeps it fresh
	setLockAge(t, lock, lockStaleTimeout-time.Minute)
	assert.False(t, lockAcquired(srv))

	// lock left behind by a run which isn't running anymore
	setLockAge(t, lock, lockStaleTimeout+time.Minute)
	assert.True(t, lockAcquired(srv))
}

func TestExecutionLock_keepFresh(t *testing.T) {
	srv, lock := newLockTestService(t)
	setLockAge(t, lock, lockStaleTimeout+time.Minute)

	stop := lock.keepFresh(5*time.Millisecond, func() { t.Error("lock reported as lost") })
	time.Sleep(50 * time.Millisecond)
	stop()

	stat, err := os.Stat(lock.path)
	assert.NoError(t, err)
	assert.True(t, time.Since(stat.ModTime()) < lockStaleTimeout)

	// the lock is refreshed, so another run can't acquire it
	assert.False(t, lockAcquired(srv))

	assert.NoError(t, lock.release())
	assert.True(t, lockAcquired(srv))
}

func TestExecutionLock_takenOver(t *testing.T) {
	srv, lock := newLockTestService(t)

	// another run takes over the lock, e.g. while this process was suspended
	setLockAge(t, lock, lockStaleTimeout+time.Minute)
	otherLock, err := srv.acquireLock()
	assert.NoError(t, err)
	setLockAge(t, otherLock, time.Minute)

	lost := make(chan struct{})
	stop := lock.keepFresh(5*time.Millisecond, func() { close(lost) })

	select {
	case <-lost:
	case <-time.After(time.Second):
		t.Fatal("lost lock not detected")
	}
	stop()

	// the other run's lock is neither refreshed nor removed
	stat, err := os.Stat(otherLock.path)
	assert.NoError(t, err)
	assert.True(t, time.Since(stat.ModTime()) > 30*time.Second)

	assert.True(t, lock.release() != nil)
	assert.NoError(t, otherLock.owned())
}

func TestService_Execute_LockLeftBehind(t *testing.T) {
	srv := New(nil, t.TempDir(), t.TempDir()).WithLockFile(filepath.Join(t.TempDir(), "config.lock"))

	// the execution timeout doesn't affect how long a lock left behind blocks configuration execution
	srv.executionTimeout.Store(int64(maximumExecutionTimeout))

	config := &CommittedConfig{
		Bundles: []string{BundleConnectivityWatchdog},
		BundleData: BundleData{
			ConnectivityWatchdog: &ConnectivityWatchdogBundle{Metadata: Metadata{Enabled: true}, Threshold: "5"},
		},
	}

	// a recently refreshed lock belongs to a running execution, so this execution is skipped
	_, err := srv.acquireLock()
	assert.NoError(t, err)

	lockPath := srv.lockFilePath()
	modTime := time.Now().Add(-time.Minute)
	assert.NoError(t, os.Chtimes(lockPath, modTime, modTime))

	assert.NoError(t, srv.Execute(t.Context(), config))
	assert.Equal(t, srv.connectivityWatchdogThreshold.Load(), int64(0))

	// a lock which wasn't refreshed for a few minutes was left behind (e.g. the agent was killed), so it's taken over
	modTime = time.Now().Add(-lockStaleTimeout - time.Minute)
	assert.NoError(t, os.Chtimes(lockPath, modTime, modTime))

	assert.NoError(t, srv.Execute(t.Context(), config))
	assert.Equal(t, srv.connectivityWatchdogThreshold.Load(), int64(5))

	// the execution released its lock
	_, err = os.Stat(lockPath)
	assert.True(t, os.IsNotExist(err))
}
