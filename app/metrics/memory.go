package metrics

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/qbee-io/qbee-agent/app/inventory/linux"
	"github.com/qbee-io/qbee-agent/app/utils"
)

// MemoryValues
//
// Example payload:
// {
//  "memtot": 32521216,
//  "memused": 5729248,
//  "memfree": 26791968,
//  "memutil": 17,
//  "swaptot": 2002940,
//  "swapused": 0,
//  "swapfree": 2002940,
//  "swaputil": 0
// }
type MemoryValues struct {
	MemoryTotal       int `json:"memtot"`
	MemoryUsed        int `json:"memused"`
	MemoryFree        int `json:"memfree"`
	MemoryUtilization int `json:"memutil"`
	SwapTotal         int `json:"swaptot"`
	SwapUsed          int `json:"swapused"`
	SwapFree          int `json:"swapfree"`
	SwapUtilization   int `json:"swaputil"`
}

func CollectMemory() ([]Metric, error) {
	path := filepath.Join(linux.ProcFS, "meminfo")

	values := new(MemoryValues)

	err := utils.ForLinesInFile(path, func(line string) error {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil
		}

		var err error

		switch fields[0] {
		case "MemTotal:":
			values.MemoryTotal, err = strconv.Atoi(fields[1])
		case "MemAvailable:":
			values.MemoryFree, err = strconv.Atoi(fields[1])
		case "SwapTotal:":
			values.SwapTotal, err = strconv.Atoi(fields[1])
		case "SwapFree:":
			values.SwapFree, err = strconv.Atoi(fields[1])
		}

		return err
	})
	if err != nil {
		return nil, err
	}

	values.MemoryUsed = values.MemoryTotal - values.MemoryFree
	if values.MemoryTotal > 0 {
		values.MemoryUtilization = values.MemoryUsed * 100 / values.MemoryTotal
	}

	values.SwapUsed = values.SwapTotal - values.SwapFree
	if values.SwapTotal > 0 {
		values.SwapUtilization = values.SwapUsed * 100 / values.SwapTotal
	}

	metric := Metric{
		Label:     Memory,
		Timestamp: time.Now().Unix(),
		Values: Values{
			MemoryValues: values,
		},
	}

	return []Metric{metric}, nil
}
