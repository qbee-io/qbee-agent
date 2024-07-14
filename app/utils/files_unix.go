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

//go:build unix

package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// DetermineFileOwner detects uid and gid for the path.
func DetermineFileOwner(dst string) (int, int, error) {
	fileInfo, err := os.Stat(dst)
	if err != nil {
		// if path doesn't exist, try to determine owner of the parent directory
		if errors.Is(err, fs.ErrNotExist) {
			parentDirPath := filepath.Dir(dst)

			if parentDirPath == dst {
				// this should never happen, but in case it does, use the process uid/gid
				return os.Geteuid(), os.Getgid(), nil
			}

			return DetermineFileOwner(parentDirPath)
		}

		return 0, 0, fmt.Errorf("cannot check file ownership: %s - %w", dst, err)
	}

	// if file exists, use its uid/gid
	fileStat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("cannot check file ownership: %s - unsupported OS", dst)
	}

	uid, gid := int(fileStat.Uid), int(fileStat.Gid)

	return uid, gid, nil
}
