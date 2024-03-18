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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.qbee.io/agent/app/inventory/linux"
	"go.qbee.io/agent/app/utils"
)

// MemoryValues contains memory metrics.
//
// Example payload:
//
//	{
//	 "memfree": 26791968,
//	 "memutil": 17,
//	 "swaputil": 0
//	}
type MemoryValues struct {
	MemoryTotal       int `json:"-"`
	MemoryUsed        int `json:"-"`
	MemoryFree        int `json:"memfree"`
	MemoryUtilization int `json:"memutil"`
	SwapTotal         int `json:"-"`
	SwapUsed          int `json:"-"`
	SwapFree          int `json:"-"`
	SwapUtilization   int `json:"swaputil"`
}

// CollectMemory metrics.
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
