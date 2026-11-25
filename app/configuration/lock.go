// Copyright 2023 qbee.io
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
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.qbee.io/agent/app/log"
)

const lockFileName = "config.lock"

const (
	// lockRefreshInterval defines how often the lock file is refreshed while configuration is executed.
	lockRefreshInterval = 30 * time.Second

	// lockStaleTimeout defines after how long without a refresh the lock file is considered stale,
	// i.e. left behind by a process which isn't running anymore.
	lockStaleTimeout = 2 * time.Minute
)

// lockFilePath returns the path to the lock file.
func (srv *Service) lockFilePath() string {
	if srv.lockFile != "" {
		return srv.lockFile
	}

	tmpfsDirs := []string{"/run", "/var/run"}

	for _, dir := range tmpfsDirs {
		if _, err := os.Stat(dir); err == nil {
			lockfileDir := filepath.Join(dir, "qbee")
			if err := os.MkdirAll(lockfileDir, 0700); err == nil {
				return filepath.Join(lockfileDir, lockFileName)
			}
		}
	}
	// no tmpfs dirs found, use app directory
	return filepath.Join(srv.appDirectory, lockFileName)
}

// executionLock is the configuration execution lock held by this process.
type executionLock struct {
	path string

	// token identifies this lock, so a lock which was taken over by another process (after it was considered
	// stale, e.g. because this process was suspended) is never refreshed or removed by this process.
	token string
}

// acquireLock for the configuration execution.
// While the lock is held, keepFresh must be running, so other processes don't consider the lock stale.
func (srv *Service) acquireLock() (*executionLock, error) {
	lockPath := srv.lockFilePath()

	// Check if lock file exists and is not stale
	if lockFileStat, err := os.Stat(lockPath); err == nil {
		lockFileExpired := time.Since(lockFileStat.ModTime()) > lockStaleTimeout

		if !lockFileExpired {
			return nil, fmt.Errorf("another process is running configuration")
		}

		if err = os.Remove(lockPath); err != nil {
			return nil, fmt.Errorf("could not remove stale lock file: %w", err)
		}
	}

	// Create lock file
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("could not create lock file: %w", err)
	}
	defer func() { _ = lockFile.Close() }()

	lock := &executionLock{
		path:  lockPath,
		token: fmt.Sprintf("%10d %d", os.Getpid(), time.Now().UnixNano()),
	}

	if _, err = lockFile.Write([]byte(lock.token)); err != nil {
		return nil, fmt.Errorf("could not write lock file: %w", err)
	}

	return lock, nil
}

// owned returns nil when the lock file still belongs to this lock.
func (lock *executionLock) owned() error {
	data, err := os.ReadFile(lock.path)
	if err != nil {
		return fmt.Errorf("could not read lock file: %w", err)
	}

	if string(data) != lock.token {
		return fmt.Errorf("lock file was taken over by another process")
	}

	return nil
}

// keepFresh refreshes the lock file every interval, until the returned stop function is called.
// When the lock is found to be taken over by another process, refreshing stops and onLost is called.
func (lock *executionLock) keepFresh(interval time.Duration, onLost func()) (stop func()) {
	done := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := lock.owned(); err != nil {
					log.Errorf("lost execution lock - %v", err)
					onLost()
					return
				}

				now := time.Now()
				if err := os.Chtimes(lock.path, now, now); err != nil {
					log.Warnf("failed to refresh execution lock - %v", err)
				}
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
	}
}

// release the lock, unless it was taken over by another process.
func (lock *executionLock) release() error {
	if err := lock.owned(); err != nil {
		return err
	}

	if err := os.Remove(lock.path); err != nil {
		return fmt.Errorf("could not remove lock file: %w", err)
	}

	return nil
}
