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

package inventory

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// parseUnameSyscall fills out system info based on results from uname syscall.
func (systemInfo *SystemInfo) parseUnameSyscall() error {
	utsname := new(unix.Utsname)

	if err := unix.Uname(utsname); err != nil {
		return fmt.Errorf("error calling Uname syscall: %w", err)
	}

	systemInfo.Host = unix.ByteSliceToString(utsname.Nodename[:])
	systemInfo.UQHost = unix.ByteSliceToString(utsname.Nodename[:])
	systemInfo.FQHost = unix.ByteSliceToString(utsname.Nodename[:])
	systemInfo.Release = unix.ByteSliceToString(utsname.Release[:])
	systemInfo.Version = unix.ByteSliceToString(utsname.Version[:])
	systemInfo.Architecture = unix.ByteSliceToString(utsname.Machine[:])
	/*
		domainName := unix.ByteSliceToString(utsname.Domainname[:])
		if domainName != "" && domainName != "(none)" {
			systemInfo.FQHost = fmt.Sprintf("%s.%s", systemInfo.UQHost, domainName)
		}
	*/
	return nil
}
