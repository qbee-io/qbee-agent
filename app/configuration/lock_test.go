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

// newLockTestService returns a service which holds the execution lock, released when the test finishes.
func newLockTestService(t *testing.T) *Service {
	srv := New(nil, t.TempDir(), t.TempDir()).WithLockFile(filepath.Join(t.TempDir(), "config.lock"))

	assert.NoError(t, srv.acquireLock())
	t.Cleanup(func() { _ = srv.releaseLock() })

	return srv
}

func setLockAge(t *testing.T, srv *Service, age time.Duration) {
	modTime := time.Now().Add(-age)
	assert.NoError(t, os.Chtimes(srv.lockFilePath(), modTime, modTime))
}

func TestService_acquireLock(t *testing.T) {
	srv := newLockTestService(t)

	// lock held by another run, which keeps it fresh
	setLockAge(t, srv, lockStaleTimeout-time.Minute)
	assert.True(t, srv.acquireLock() != nil)

	// lock left behind by a run which isn't running anymore
	setLockAge(t, srv, lockStaleTimeout+time.Minute)
	assert.NoError(t, srv.acquireLock())
}

func TestService_keepLockFresh(t *testing.T) {
	srv := newLockTestService(t)
	setLockAge(t, srv, lockStaleTimeout+time.Minute)

	stop := srv.keepLockFresh(5 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	stop()

	stat, err := os.Stat(srv.lockFilePath())
	assert.NoError(t, err)
	assert.True(t, time.Since(stat.ModTime()) < lockStaleTimeout)

	// the lock is refreshed, so another run can't acquire it
	assert.True(t, srv.acquireLock() != nil)
}
