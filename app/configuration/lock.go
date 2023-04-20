package configuration

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const lockFileName = "config.lock"
const lockFileTimeout = time.Hour

// lockFilePath returns the path to the lock file.
func (srv *Service) lockFilePath() string {
	return filepath.Join(srv.appDirectory, lockFileName)
}

// acquireLock for the configuration execution.
func (srv *Service) acquireLock() error {
	// Check if lock file exists and is not expired
	if lockFileStat, err := os.Stat(srv.lockFilePath()); err == nil {
		lockFileExpired := time.Since(lockFileStat.ModTime()) > lockFileTimeout

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
	defer lockFile.Close()

	lockFileData := fmt.Sprintf("%10d", os.Getpid())
	if _, err = lockFile.Write([]byte(lockFileData)); err != nil {
		return fmt.Errorf("could not write lock file: %w", err)
	}

	return nil
}

// releaseLock for the configuration execution.
func (srv *Service) releaseLock() error {
	if err := os.Remove(srv.lockFilePath()); err != nil {
		return fmt.Errorf("could not remove lock file: %w", err)
	}

	return nil
}
