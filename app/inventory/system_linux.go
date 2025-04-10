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
	"syscall"
	"time"
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
