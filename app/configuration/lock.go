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

// acquireLock for the configuration execution.
// While the lock is held, keepLockFresh must be running, so other processes don't consider the lock stale.
func (srv *Service) acquireLock() error {
	// Check if lock file exists and is not stale
	if lockFileStat, err := os.Stat(srv.lockFilePath()); err == nil {
		lockFileExpired := time.Since(lockFileStat.ModTime()) > lockStaleTimeout

		if !lockFileExpired {
			return fmt.Errorf("another process is running configuration")
		}

		if err = srv.releaseLock(); err != nil {
			return err
		}
	}

	// Create lock file
	lockFile, err := os.OpenFile(srv.lockFilePath(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not create lock file: %w", err)
	}
	defer func() { _ = lockFile.Close() }()

	lockFileData := fmt.Sprintf("%10d", os.Getpid())
	if _, err = lockFile.Write([]byte(lockFileData)); err != nil {
		return fmt.Errorf("could not write lock file: %w", err)
	}

	return nil
}

// keepLockFresh refreshes the lock file every interval, until the returned stop function is called.
func (srv *Service) keepLockFresh(interval time.Duration) (stop func()) {
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
				now := time.Now()
				if err := os.Chtimes(srv.lockFilePath(), now, now); err != nil {
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

// releaseLock for the configuration execution.
func (srv *Service) releaseLock() error {
	if err := os.Remove(srv.lockFilePath()); err != nil {
		return fmt.Errorf("could not remove lock file: %w", err)
	}

	return nil
}
