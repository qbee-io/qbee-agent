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

//go:build freebsd || openbsd || netbsd

package inventory

import (
	"context"
	"fmt"

	"golang.org/x/sys/unix"
)

func (systemInfo *SystemInfo) parseSysinfoSyscall() error {
	tv, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return err
	}

	systemInfo.BootTime = fmt.Sprintf("%d", tv.Sec)

	return nil
}

func (systemInfo *SystemInfo) getDefaultNetworkInterface(ctx context.Context) (string, error) {
	// not implemented for BSD systems
	return "", nil
}

// parseCPUInfo parses /proc/cpuinfo for extra details re. CPU.
func (systemInfo *SystemInfo) parseCPUInfo(ctx context.Context) error {
	// not implemented for BSD systems
	return nil
}
