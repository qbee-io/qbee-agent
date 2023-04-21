package configuration

import (
	"fmt"
	"os"
	"time"
)

const lockFilePath = "/run/lock/LCK..qbee-agent-configuration"

// acquireLock for the configuration execution.
func acquireLock(lockFileTimeout time.Duration) error {
	// Check if lock file exists and is not expired
	if lockFileStat, err := os.Stat(lockFilePath); err == nil {
		lockFileExpired := time.Since(lockFileStat.ModTime()) > lockFileTimeout

		if !lockFileExpired {
			return fmt.Errorf("another process is running configuration")
		}

		if err = releaseLock(); err != nil {
			return err
		}
	}

	// Create lock file
	lockFile, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
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
func releaseLock() error {
	if err := os.Remove(lockFilePath); err != nil {
		return fmt.Errorf("could not remove lock file: %w", err)
	}

	return nil
}
