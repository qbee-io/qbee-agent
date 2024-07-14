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
	return nil, nil
}
