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

//go:build linux

package linux

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"go.qbee.io/agent/app/utils"
)

// MemInfo provides basic information about system memory.
// See `man proc` -> `/proc/meminfo` section for details.q
type MemInfo struct {
	// TotalMemory in Kibibytes
	TotalMemory uint64

	// AvailableMemory in Kibibytes
	AvailableMemory uint64
}

// GetMemInfo returns basic memory information from the system.
func GetMemInfo() (*MemInfo, error) {
	filePath := filepath.Join(ProcFS, "meminfo")

	memInfo := new(MemInfo)

	err := utils.ForLinesInFile(filePath, func(line string) error {
		var err error

		fields := strings.Fields(line)

		switch fields[0] {
		case "MemTotal:":
			if len(fields) != 3 || fields[2] != "kB" {
				return fmt.Errorf("unsupported file format")
			}

			memInfo.TotalMemory, err = strconv.ParseUint(fields[1], 10, 64)
		case "MemAvailable:":
			if len(fields) != 3 || fields[2] != "kB" {
				return fmt.Errorf("unsupported file format")
			}

			memInfo.AvailableMemory, err = strconv.ParseUint(fields[1], 10, 64)
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	return memInfo, nil
}
