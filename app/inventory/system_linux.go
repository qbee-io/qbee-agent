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

package inventory

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.qbee.io/agent/app/inventory/linux"
	"go.qbee.io/agent/app/utils"
)

// parseSysinfoSyscall populates system info from sysinfo system call.
func (systemInfo *SystemInfo) parseSysinfoSyscall() error {
	now := time.Now()
	sysinfo, err := getSysinfo()
	if err != nil {
		return err
	}

	systemInfo.BootTime = fmt.Sprintf("%d", now.Unix()-int64(sysinfo.Uptime))

	return nil
}

// getSysinfo returns sysinfo struct.
func getSysinfo() (*syscall.Sysinfo_t, error) {
	sysinfo := new(syscall.Sysinfo_t)
	if err := syscall.Sysinfo(sysinfo); err != nil {
		return nil, fmt.Errorf("error calling sysinfo syscall: %w", err)
	}

	return sysinfo, nil
}

// getDefaultNetworkInterface returns a default network interface name.
func (systemInfo *SystemInfo) getDefaultNetworkInterface() (string, error) {

	routeFilePath := filepath.Join(linux.ProcFS, "net", "route")

	defaultInterface := ""

	err := utils.ForLinesInFile(routeFilePath, func(line string) error {
		fields := strings.Fields(line)
		if fields[1] == "Destination" {
			return nil
		}

		if defaultInterface == "" {
			defaultInterface = fields[0]
		}

		if fields[1] == "00000000" && defaultInterface == "" {
			defaultInterface = fields[0]
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("error getting default network interface: %w", err)
	}

	return defaultInterface, nil
}

// parseCPUInfo parses /proc/cpuinfo for extra details re. CPU.
func (systemInfo *SystemInfo) parseCPUInfo() error {

	filePath := filepath.Join(linux.ProcFS, "cpuinfo")

	const expectedLineSubstrings = 2

	return utils.ForLinesInFile(filePath, func(line string) error {
		line = strings.TrimSpace(line)

		substrings := strings.SplitN(line, ":", expectedLineSubstrings)
		if len(substrings) != expectedLineSubstrings {
			return nil
		}

		key := strings.TrimSpace(substrings[0])

		switch key {
		case "Serial":
			systemInfo.CPUSerialNumber = strings.TrimSpace(substrings[1])
		case "Hardware":
			systemInfo.CPUHardware = strings.TrimSpace(substrings[1])
		case "Revision":
			systemInfo.CPURevision = strings.TrimSpace(substrings[1])
		}

		return nil
	})
}
