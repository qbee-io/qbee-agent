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
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

// newLockTestService returns a service using its own lock file, which it holds.
func newLockTestService(t *testing.T) *Service {
	srv := New(nil, t.TempDir(), t.TempDir()).WithLockFile(filepath.Join(t.TempDir(), "config.lock"))

	assert.NoError(t, srv.acquireLock(lockStaleTimeout))

	return srv
}

func setLockAge(t *testing.T, srv *Service, age time.Duration) {
	modTime := time.Now().Add(-age)
	assert.NoError(t, os.Chtimes(srv.lockFilePath(), modTime, modTime))
}

func lockAcquired(srv *Service) bool {
	return srv.acquireLock(lockStaleTimeout) == nil
}

func TestService_acquireLock(t *testing.T) {
	srv := newLockTestService(t)

	// lock held by another run, which keeps it fresh
	setLockAge(t, srv, lockStaleTimeout-time.Minute)
	assert.False(t, lockAcquired(srv))

	// lock left behind by a run which isn't running anymore
	setLockAge(t, srv, lockStaleTimeout+time.Minute)
	assert.True(t, lockAcquired(srv))
}

func TestService_acquireLock_ClockSetBack(t *testing.T) {
	srv := newLockTestService(t)

	// lock refreshed by a running process just before the clock was set back slightly
	setLockAge(t, srv, -time.Minute)
	assert.False(t, lockAcquired(srv))

	// lock left behind before the clock was set back by days (e.g. after booting with a wrong time)
	setLockAge(t, srv, -72*time.Hour)
	assert.True(t, lockAcquired(srv))
}

func TestService_keepLockFresh(t *testing.T) {
	srv := newLockTestService(t)
	setLockAge(t, srv, lockStaleTimeout+time.Minute)

	ctx, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		srv.keepLockFresh(ctx, 5*time.Millisecond)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	stop()
	<-done

	// the lock is refreshed, so another run can't acquire it
	assert.False(t, lockAcquired(srv))

	assert.NoError(t, srv.releaseLock())
	assert.True(t, lockAcquired(srv))
}

func TestService_Execute_LockLeftBehind(t *testing.T) {
	srv := newLockTestService(t)

	// the execution timeout doesn't affect how long a lock left behind blocks configuration execution
	srv.executionTimeout.Store(int64(maximumExecutionTimeout))

	config := newWatchdogConfig()

	// a recently refreshed lock belongs to a running execution, so this execution is skipped
	setLockAge(t, srv, time.Minute)

	assert.NoError(t, srv.Execute(t.Context(), config))
	assert.Equal(t, srv.connectivityWatchdogThreshold.Load(), int64(0))

	// a lock which wasn't refreshed for a few minutes was left behind (e.g. the agent was killed), so it's taken over
	setLockAge(t, srv, lockStaleTimeout+time.Minute)

	assert.NoError(t, srv.Execute(t.Context(), config))
	assert.Equal(t, srv.connectivityWatchdogThreshold.Load(), int64(5))

	// the execution released its lock
	_, err := os.Stat(srv.lockFilePath())
	assert.True(t, os.IsNotExist(err))
}

// newWatchdogConfig returns a configuration which sets the connectivity watchdog threshold to 5 when executed.
func newWatchdogConfig() *CommittedConfig {
	return &CommittedConfig{
		Bundles: []string{BundleConnectivityWatchdog},
		BundleData: BundleData{
			ConnectivityWatchdog: &ConnectivityWatchdogBundle{Metadata: Metadata{Enabled: true}, Threshold: "5"},
		},
	}
}
