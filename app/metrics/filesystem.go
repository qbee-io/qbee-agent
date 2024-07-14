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

package metrics

import (
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.qbee.io/agent/app/inventory/linux"
	"go.qbee.io/agent/app/utils"
)

// FilesystemValues represents filesystem metric values.
//
// Example payload:
//
//	{
//	 "avail": 399848008,
//	 "use": 14,
//	}
type FilesystemValues struct {
	Available uint64 `json:"avail"`
	Use       uint64 `json:"use"`
}

const fsBlockSize = 1024

// CollectFilesystem returns filesystem metric for each filesystem mounted in read-write mode.
func CollectFilesystem() ([]Metric, error) {
	mounts, err := getFilesystemMounts()
	if err != nil {
		return nil, err
	}

	metrics := make([]Metric, len(mounts))

	for i, mount := range mounts {
		var st syscall.Statfs_t

		if err = syscall.Statfs(mount, &st); err != nil {
			return nil, err
		}

		size := uint64(st.Blocks) * uint64(st.Bsize) / fsBlockSize
		free := uint64(st.Bavail) * uint64(st.Bsize) / fsBlockSize

		var use uint64

		if size > 0 {
			use = 100 - (100*free)/size
		}

		metrics[i] = Metric{
			Label:     Filesystem,
			Timestamp: time.Now().Unix(),
			ID:        mount,
			Values: Values{
				FilesystemValues: &FilesystemValues{
					Available: free,
					Use:       use,
				},
			},
		}
	}

	return metrics, nil
}

// getFilesystemMounts returns a list of block-device mount points.
func getFilesystemMounts() ([]string, error) {
	supportedFilesystems, err := getSupportedFilesystems()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(linux.ProcFS, "mounts")

	mounts := make([]string, 0)

	err = utils.ForLinesInFile(path, func(line string) error {
		fields := strings.Fields(line)

		if fields[3] != "rw" && !strings.HasPrefix(fields[3], "rw,") {
			return nil
		}

		if supportedFilesystems[fields[2]] {
			mounts = append(mounts, fields[1])
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return mounts, nil
}

// getSupportedFilesystems returns a map of supported block-device filesystems.
func getSupportedFilesystems() (map[string]bool, error) {
	path := filepath.Join(linux.ProcFS, "filesystems")

	filesystems := make(map[string]bool)

	err := utils.ForLinesInFile(path, func(line string) error {
		if strings.HasPrefix(line, "nodev") {
			return nil
		}

		filesystems[strings.TrimSpace(line)] = true

		return nil
	})
	if err != nil {
		return nil, err
	}

	return filesystems, nil
}
