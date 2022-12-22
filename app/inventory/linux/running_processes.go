//go:build linux

package linux

import (
	"github.com/qbee-io/qbee-agent/app/utils"
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
