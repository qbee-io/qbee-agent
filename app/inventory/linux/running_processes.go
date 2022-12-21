//go:build linux

package linux

import (
	"fmt"
	"os"
)

// ListRunningProcesses returns a list of PIDs of currently running processes.
func ListRunningProcesses() ([]string, error) {
	procfsDir, err := os.Open(ProcFS)
	if err != nil {
		return nil, fmt.Errorf("error openning procfs: %w", err)
	}

	var dirNames []string
	if dirNames, err = procfsDir.Readdirnames(-1); err != nil {
		return nil, fmt.Errorf("error listing contents of procfs: %w", err)
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
