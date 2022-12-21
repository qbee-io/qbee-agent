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
