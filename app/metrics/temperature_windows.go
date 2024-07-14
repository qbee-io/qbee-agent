// Copyright 2024 qbee.io
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

// TemperatureValues represents temperature metrics
type TemperatureValues struct {
	// Temperature in degrees Celsius
	Temperature float64 `json:"temperature"`
}

// cpuTemperatures represents temperature metrics
type cpuTemperatures struct {
	main    float64
	cores   []float64
	socket  []float64
	chipset float64
}

// hostTemperatureScale is the scale of the temperature values (milli-degrees Celsius)
const hostTemperatureScale = 1000.0

// CollectTemperature collects temperature metrics from from /sys/class/[hwmon|thermal]
func CollectTemperature() ([]Metric, error) {
	return nil, nil
}
