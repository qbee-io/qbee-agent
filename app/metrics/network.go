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

// NetworkValues contains network metrics for an interface.
//
// Example payload:
//
//	{
//	 "label": "network",
//	 "ts": 1669988326,
//	 "id": "eth0",
//	 "values": {
//	   "tx_bytes": 7126,
//	   "rx_bytes": 17423
//	 }
//	}
type NetworkValues struct {
	// Received bytes on a network interface
	RXBytes uint64 `json:"rx_bytes"`
	// Transferred bytes on a network interface
	TXBytes uint64 `json:"tx_bytes"`
}

// CollectNetwork metrics.
// Note: collected are total values. The agent must report delta,
// so we need to keep state from the last report and subtract it before delivery.
func CollectNetwork() ([]Metric, error) {
	path := filepath.Join(linux.ProcFS, "net", "dev")

	metrics := make([]Metric, 0)

	err := utils.ForLinesInFile(path, func(line string) error {
		fields := strings.Fields(line)

		if !strings.HasSuffix(fields[0], ":") {
			return nil
		}

		ifaceName := strings.TrimSuffix(fields[0], ":")

		rxBytes, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return err
		}

		var txBytes uint64
		if txBytes, err = strconv.ParseUint(fields[9], 10, 64); err != nil {
			return err
		}

		metric := Metric{
			Label:     Network,
			Timestamp: time.Now().Unix(),
			ID:        ifaceName,
			Values: Values{
				NetworkValues: &NetworkValues{
					RXBytes: rxBytes,
					TXBytes: txBytes,
				},
			},
		}

		metrics = append(metrics, metric)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// Delta calculates the delta between two NetworkValues.
func (v *NetworkValues) Delta(old *NetworkValues) (*NetworkValues, error) {
	if old == nil {
		return v, nil
	}

	rxBytes := uint64(0)
	if v.RXBytes > old.RXBytes {
		rxBytes = v.RXBytes - old.RXBytes
	}

	txBytes := uint64(0)
	if v.TXBytes > old.TXBytes {
		txBytes = v.TXBytes - old.TXBytes
	}

	return &NetworkValues{
		RXBytes: rxBytes,
		TXBytes: txBytes,
	}, nil
}
