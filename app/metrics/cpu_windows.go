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

// CPUValues contains CPU metrics.
//
// Example payload:
//
//	{
//	 "user": 2.08,
//	 "system": 0.76,
//	 "iowait": 0.00,
//	}
type CPUValues struct {
	User   float64 `json:"user"`
	Nice   float64 `json:"-"`
	System float64 `json:"system"`
	Idle   float64 `json:"-"`
	IOWait float64 `json:"iowait"`
	IRQ    float64 `json:"-"`
}

// CollectCPU returns CPU metrics.
func CollectCPU() (*CPUValues, error) {
	return nil, nil
}

func (c *CPUValues) Delta(previous *CPUValues) (*CPUValues, error) {
	return nil, nil
}
