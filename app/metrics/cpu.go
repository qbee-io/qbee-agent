package metrics

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/qbee-io/qbee-agent/app/inventory/linux"
)

// CPUValues
//
// Example payload:
//
//	{
//	 "user": 2.08,
//	 "nice": 0.00,
//	 "system": 0.76,
//	 "idle": 97.16,
//	 "iowait": 0.00,
//	 "irq": 0.00
//	}
type CPUValues struct {
	User   float64 `json:"user"`
	Nice   float64 `json:"nice"`
	System float64 `json:"system"`
	Idle   float64 `json:"idle"`
	IOWait float64 `json:"iowait"`
	IRQ    float64 `json:"irq"`
}

// CollectCPU returns CPU metrics.
func CollectCPU() ([]Metric, error) {
	filePath := filepath.Join(linux.ProcFS, "stat")

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filePath, err)
	}

	defer file.Close()

	// we don't need to read the whole file, we only care about the first line
	buf := make([]byte, 512)
	if _, err = file.Read(buf); err != nil {
		return nil, fmt.Errorf("error reading contents of %s: %w", filePath, err)
	}

	firstLine := string(buf[0:bytes.Index(buf, []byte("\n"))])
	lineFields := strings.Fields(firstLine)

	var total uint64
	fields := []string{"user", "nice", "system", "idle", "iowait", "irq"}
	fieldValues := make(map[string]uint64)

	for i, field := range fields {
		if fieldValues[field], err = strconv.ParseUint(lineFields[i+1], 10, 64); err != nil {
			return nil, fmt.Errorf("error parsing %s field in %s: %w", field, filePath, err)
		}

		total += fieldValues[field]
	}

	metric := Metric{
		Label:     CPU,
		Timestamp: time.Now().Unix(),
		Values: Values{
			CPUValues: &CPUValues{
				User:   float64(fieldValues["user"]),
				Nice:   float64(fieldValues["nice"]),
				System: float64(fieldValues["system"]),
				Idle:   float64(fieldValues["idle"]),
				IOWait: float64(fieldValues["iowait"]),
				IRQ:    float64(fieldValues["irq"]),
			},
		},
	}

	return []Metric{metric}, nil
}
