//go:build linux

package linux

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/qbee-io/qbee-agent/app/utils"
)

// MemInfo provides basic information about system memory.
// See `man proc` -> `/proc/meminfo` section for details.q
type MemInfo struct {
	// TotalMemory in Kibibytes
	TotalMemory uint64

	// AvailableMemory in Kibibytes
	AvailableMemory uint64
}

// GetMemInfo returns basic memory information from the system.
func GetMemInfo() (*MemInfo, error) {
	filePath := path.Join(ProcFS, "meminfo")

	memInfo := new(MemInfo)

	err := utils.ForLinesInFile(filePath, func(line string) error {
		var err error

		fields := strings.Fields(line)

		switch fields[0] {
		case "MemTotal:":
			if len(fields) != 3 || fields[2] != "kB" {
				return fmt.Errorf("unsupported file format")
			}

			memInfo.TotalMemory, err = strconv.ParseUint(fields[1], 10, 64)
		case "MemAvailable:":
			if len(fields) != 3 || fields[2] != "kB" {
				return fmt.Errorf("unsupported file format")
			}

			memInfo.AvailableMemory, err = strconv.ParseUint(fields[1], 10, 64)
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	return memInfo, nil
}
