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
	"os"
	"path/filepath"
	"strings"

	"go.qbee.io/agent/app/utils"
)

// ListRunningProcesses returns a list of PIDs of currently running processes.
func ListRunningProcesses() ([]string, error) {
	dirNames, err := utils.ListDirectory(ProcFS)
	if err != nil {
		return nil, err
	}

	// return only directories with numeric filename
	result := make([]string, 0, len(dirNames))
	for _, dirName := range dirNames {
		if dirName[0] < '0' || dirName[0] > '9' {
			continue
		}

		result = append(result, dirName)
	}

	return result, nil
}

// ListRunningProcessesNames returns a map of running process PID -> process command-line.
func ListRunningProcessesNames() (map[string]string, error) {
	runningProcesses, err := ListRunningProcesses()
	if err != nil {
		return nil, err
	}

	processMap := make(map[string]string)

	for _, pid := range runningProcesses {
		var cmdLine string

		if cmdLine, err = GetProcessCommand(pid); err != nil {
			return nil, err
		}

		processMap[pid] = cmdLine
	}

	return processMap, nil
}

// GetProcessCommand returns a command used to start the process.
func GetProcessCommand(pid string) (string, error) {
	cmdLinePath := filepath.Join(ProcFS, pid, "cmdline")

	cmdLineBytes, err := os.ReadFile(cmdLinePath)
	if err != nil {
		return "", fmt.Errorf("error reading %s: %w", cmdLinePath, err)
	}

	// cleanup the command line and replace null-bytes with spaces
	cmdLine := strings.TrimSpace(strings.ReplaceAll(string(cmdLineBytes), "\000", " "))

	return cmdLine, nil
}
