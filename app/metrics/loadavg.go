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

package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.qbee.io/agent/app/inventory/linux"
)

// LoadAverageValues contains load average metrics.
//
// Example payload:
//
//	{
//	 "1min": 1.17,
//	 "5min": 0.84,
//	 "15min": 0.77
//	}
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
