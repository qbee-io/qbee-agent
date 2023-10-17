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
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/qbee-io/qbee-agent/app/utils"
)

type ProcessStatus struct {
	Owner  string
	Memory uint64
}

// GetProcessStatus returns ProcessStatus based on /proc/*/status.
// See `man proc` -> `/proc/[pid]/status section for details on the file format.
func GetProcessStatus(pid string) (*ProcessStatus, error) {
	statusFilePath := filepath.Join(ProcFS, pid, "status")
	processStatus := new(ProcessStatus)

	err := utils.ForLinesInFile(statusFilePath, func(line string) error {
		fields := strings.Fields(line)

		switch fields[0] {
		case "Uid:":
			effectiveUID := fields[3]

			userInfo, err := user.LookupId(effectiveUID)
			if err != nil {
				// if user lookup fails, use UID for Username
				userInfo = &user.User{Username: effectiveUID}
			}

			processStatus.Owner = userInfo.Username
		case "RssAnon:", "RssFile:", "RssShmem:":
			if len(fields) != 3 || fields[2] != "kB" {
				return fmt.Errorf("unsupported file format")
			}

			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return err
			}

			processStatus.Memory += value
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return processStatus, nil
}
