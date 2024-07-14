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

//go:build windows

package metrics

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

func CollectNetwork() ([]Metric, error) {
	return nil, nil
}

// Delta calculates the delta between two NetworkValues.
func (v *NetworkValues) Delta(old *NetworkValues) (*NetworkValues, error) {
	return nil, nil
}
