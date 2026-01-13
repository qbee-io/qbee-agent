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

package agent

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/log"
)

const (
	directoryMode        = 0700
	credentialsDirectory = "ppkeys"
	appWorkingDirectory  = "app_workdir"
	cacheDirectory       = "cache"
)

// prepareConfigDirectories makes sure that agent's directories are in place.
func prepareConfigDirectories(cfgDirectory string) error {
	log.Infof("Preparing agent directories")

	directories := []string{
		cfgDirectory,
		filepath.Join(cfgDirectory, credentialsDirectory),
	}

	return prepareDirectories(directories, os.Geteuid(), os.Getegid())
}

// prepareStateDirectories makes sure that agent's state directories are in place.
func prepareStateDirectories(stateDirectory string, owner, group int) error {
	cacheDirectoryPath := filepath.Join(stateDirectory, appWorkingDirectory, cacheDirectory)
	directories := []string{
		stateDirectory,
		filepath.Join(stateDirectory, appWorkingDirectory),
		cacheDirectoryPath,
		filepath.Join(cacheDirectoryPath, configuration.FileDistributionCacheDirectory),
		filepath.Join(cacheDirectoryPath, configuration.SoftwareCacheDirectory),
		filepath.Join(cacheDirectoryPath, configuration.DockerContainerDirectory),
		filepath.Join(cacheDirectoryPath, configuration.PodmanContainerDirectory),
		filepath.Join(cacheDirectoryPath, configuration.DockerComposeDirectory),
	}

	return prepareDirectories(directories, owner, group)
}

// prepareDirectories creates required directories with correct permissions.
func prepareDirectories(directories []string, owner, group int) error {
	for _, directory := range directories {
		if err := createDirectory(directory, owner, group); err != nil {
			return err
		}
	}

	return nil
}

// createDirectory checks whether directory exists and has correct permissions.
// Directory will be created if not found.
func createDirectory(path string, owner, group int) error {
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
		if err = os.Chmod(path, directoryMode); err != nil {
			return fmt.Errorf("directory %s has incorrect permissions %s and unable to fix: %w,", path, stats.Mode(), err)
		}
	}

	// walk directory and set ownership if necessary
	err = filepath.Walk(path, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, owner, group)
	})
	if err != nil {
		return fmt.Errorf("error setting ownership for directory %s: %w", path, err)
	}

	return nil
}
