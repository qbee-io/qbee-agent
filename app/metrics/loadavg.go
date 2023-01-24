package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/qbee-io/qbee-agent/app/inventory/linux"
)

// LoadAverageValues
//
// Example payload:
//
// {
//  "1min": 1.17,
//  "5min": 0.84,
//  "15min": 0.77
// }
type LoadAverageValues struct {
	// Minute1 average system load over 1 minute.
	Minute1 float64 `json:"1min"`

	// Minute1 average system load over 5 minutes.
	Minute5 float64 `json:"5min"`

	// Minute1 average system load over 15 minutes.
	Minute15 float64 `json:"15min"`
}

// CollectLoadAverage metrics.
func CollectLoadAverage() ([]Metric, error) {
	path := filepath.Join(linux.ProcFS, "loadavg")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", path, err)
	}

	fields := strings.Fields(string(data))

	metric := Metric{
		Label:     LoadAverage,
		Timestamp: time.Now().Unix(),
		Values: Values{
			LoadAverageValues: &LoadAverageValues{
				Minute1:  0,
				Minute5:  0,
				Minute15: 0,
			},
		},
	}

	if metric.Values.Minute1, err = strconv.ParseFloat(fields[0], 64); err != nil {
		return nil, fmt.Errorf("failed to parse 1 minute average: %w", err)
	}

	if metric.Values.Minute5, err = strconv.ParseFloat(fields[1], 64); err != nil {
		return nil, fmt.Errorf("failed to parse 5 minute average: %w", err)
	}

	if metric.Values.Minute15, err = strconv.ParseFloat(fields[2], 64); err != nil {
		return nil, fmt.Errorf("failed to parse 15 minute average: %w", err)
	}

	return []Metric{metric}, nil
}
