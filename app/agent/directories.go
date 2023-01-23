package agent

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/qbee-io/qbee-agent/app/configuration"
	"github.com/qbee-io/qbee-agent/app/log"
)

const (
	directoryMode        = 0700
	credentialsDirectory = "ppkeys"
	appWorkingDirectory  = "app_workdir"
	cacheDirectory       = "cache"
)

// prepareDirectories makes sure that agent's directories are in place.
func prepareDirectories(cfgDirectory, stateDirectory string) error {
	log.Infof("Preparing agent directories")

	cacheDirectoryPath := filepath.Join(stateDirectory, appWorkingDirectory, cacheDirectory)

	directories := []string{
		filepath.Join(cfgDirectory, credentialsDirectory),
		filepath.Join(cacheDirectoryPath, configuration.FileDistributionCacheDirectory),
		filepath.Join(cacheDirectoryPath, configuration.SoftwareCacheDirectory),
		filepath.Join(cacheDirectoryPath, configuration.DockerContainerDirectory),
	}

	for _, directory := range directories {
		if err := createDirectory(directory); err != nil {
			return err
		}
	}

	return nil
}

// createDirectory checks whether directory exists and has correct permissions.
// Directory will be created if not found.
func createDirectory(path string) error {
	if err := os.MkdirAll(path, directoryMode); err != nil {
		return fmt.Errorf("error creating directory %s: %w", path, err)
	}

	stats, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error checking status of directory %s: %w", path, err)
	}

	if !stats.IsDir() {
		return fmt.Errorf("path %s is not a directory", path)
	}

	if stats.Mode() != directoryMode|fs.ModeDir {
		return fmt.Errorf("directory %s has incorrect permissions %s", path, stats.Mode())
	}

	return nil
}
