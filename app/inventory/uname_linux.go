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

//go:build linux && !arm

package inventory

import (
	"fmt"
	"syscall"
)

// parseUnameSyscall fills out system info based on results from uname syscall.
func (systemInfo *SystemInfo) parseUnameSyscall() error {
	utsname := new(syscall.Utsname)

	if err := syscall.Uname(utsname); err != nil {
		return fmt.Errorf("error calling Uname syscall: %w", err)
	}

	systemInfo.Host = int8SliceToString(utsname.Nodename[:])
	systemInfo.UQHost = int8SliceToString(utsname.Nodename[:])
	systemInfo.FQHost = int8SliceToString(utsname.Nodename[:])
	systemInfo.Release = int8SliceToString(utsname.Release[:])
	systemInfo.Version = int8SliceToString(utsname.Version[:])
	systemInfo.Architecture = int8SliceToString(utsname.Machine[:])

	domainName := int8SliceToString(utsname.Domainname[:])
	if domainName != "" && domainName != "(none)" {
		systemInfo.FQHost = fmt.Sprintf("%s.%s", systemInfo.UQHost, domainName)
	}

	return nil
}

// int8SliceToString converts slice []int8 into a string.
func int8SliceToString(val []int8) string {
	buf := make([]byte, 0, len(val))
	for _, b := range val {
		if b == 0 {
			break
		}

		buf = append(buf, byte(b))
	}

	return string(buf[:])
}
