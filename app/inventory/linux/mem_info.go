//go:build linux

package linux

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
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

	fp, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %w", filePath, err)
	}

	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	memInfo := new(MemInfo)

	for scanner.Scan() {
		if err = scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading line from %s: %w", filePath, err)
		}

		fields := strings.Fields(scanner.Text())

		switch fields[0] {
		case "MemTotal:":
			if len(fields) != 3 || fields[2] != "kB" {
				return nil, fmt.Errorf("unsupported file format: %s", filePath)
			}

			memInfo.TotalMemory, err = strconv.ParseUint(fields[1], 10, 64)
		case "MemAvailable:":
			if len(fields) != 3 || fields[2] != "kB" {
				return nil, fmt.Errorf("unsupported file format: %s", filePath)
			}

			memInfo.AvailableMemory, err = strconv.ParseUint(fields[1], 10, 64)
		}

		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", filePath, err)
		}
	}

	return memInfo, nil
}
