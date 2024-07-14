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
	return nil, nil
}
