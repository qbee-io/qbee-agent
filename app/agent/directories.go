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

	"github.com/qbee-io/qbee-agent/app/configuration"
	"github.com/qbee-io/qbee-agent/app/log"
)

const (
	directoryMode        = 0700
	credentialsDirectory = "ppkeys"
	appWorkingDirectory  = "app_workdir"
	cacheDirectory       = "cache"
	binDirectory         = "bin"
)

// prepareDirectories makes sure that agent's directories are in place.
func prepareDirectories(cfgDirectory, stateDirectory string) error {
	log.Infof("Preparing agent directories")

	cacheDirectoryPath := filepath.Join(stateDirectory, appWorkingDirectory, cacheDirectory)

	directories := []string{
		filepath.Join(cfgDirectory, credentialsDirectory),
		filepath.Join(stateDirectory, appWorkingDirectory, binDirectory),
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
		if err = os.Chmod(path, directoryMode); err != nil {
			return fmt.Errorf("directory %s has incorrect permissions %s and unable to fix: %w,", path, stats.Mode(), err)
		}
	}

	return nil
}
