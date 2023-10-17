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
	"strconv"
	"strings"
)

const (
	processStatsFieldPID                    = 0
	processStatsFieldCmd                    = 1
	processStatsFieldUserModeJiffies        = 13
	processStatsFieldKernelModeJiffies      = 14
	processStatsFieldChildUserModeJiffies   = 15
	processStatsFieldChildKernelModeJiffies = 16
)

// NewProcessStats returns ProcessStats based on provided /proc/[pid]/stat file contents.
func NewProcessStats(line string) (ProcessStats, error) {
	cmdIndexStart := strings.Index(line, "(")
	cmdIndexEnd := strings.Index(line, ")")

	if cmdIndexStart < 0 || cmdIndexEnd < 0 {
		return nil, fmt.Errorf("unexpected stat file format")
	}

	pid := line[0 : cmdIndexStart-1]
	command := line[cmdIndexStart+1 : cmdIndexEnd]
	remainingFields := strings.Fields(line[cmdIndexEnd+2:])

	ps := append([]string{pid, command}, remainingFields...)

	return ps, nil
}

// ProcessStats represents process stats file (/proc/*/stat).
// See `man proc` -> `/proc/[pid]/stat` section for details of the file format.
type ProcessStats []string

// String returns PID of the process.
func (ps ProcessStats) String() string {
	return ps[processStatsFieldPID]
}

// PID returns process ID as integer.
func (ps ProcessStats) PID() int {
	pid, _ := strconv.Atoi(ps[processStatsFieldPID])

	return pid
}

// Command returns processes command.
func (ps ProcessStats) Command() string {
	return strings.Trim(ps[processStatsFieldCmd], "()")
}

// GetJiffies returns sum of process' Jiffies.
func (ps ProcessStats) GetJiffies() (uint64, error) {
	var err error
	var utime, stime, cutime, cstime uint64

	if utime, err = strconv.ParseUint(ps[processStatsFieldUserModeJiffies], 10, 64); err != nil {
		return 0, fmt.Errorf("error parsing utime for process %s: %w", ps, err)
	}

	if stime, err = strconv.ParseUint(ps[processStatsFieldKernelModeJiffies], 10, 64); err != nil {
		return 0, fmt.Errorf("error parsing stime for process %s: %w", ps, err)
	}

	if cutime, err = strconv.ParseUint(ps[processStatsFieldChildUserModeJiffies], 10, 64); err != nil {
		return 0, fmt.Errorf("error parsing cutime for process %s: %w", ps, err)
	}

	if cstime, err = strconv.ParseUint(ps[processStatsFieldChildKernelModeJiffies], 10, 64); err != nil {
		return 0, fmt.Errorf("error parsing cstime for process %s: %w", ps, err)
	}

	jiffies := utime + stime + cutime + cstime

	return jiffies, nil
}
