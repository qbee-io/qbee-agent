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

package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type TemperatureValues struct {
	// Temperature in degrees Celsius
	Temperature float64 `json:"temperature"`
}

const sysFS = "/sys"
const hostTemperatureScale = 1000.0

func CollectTemperature() ([]Metric, error) {

	// Attempt to collect temperature metrics from hwmon
	if metrics, err := hwMonTemperatureMetrics(); err == nil {
		if len(metrics) > 0 {
			return metrics, nil
		}
	}

	// Attempt to collect temperature metrics from thermal zone
	if metrics, err := thermalZoneTemperatureMetrics(); err == nil {
		if len(metrics) > 0 {
			return metrics, nil
		}
	}

	// If no temperature metrics were found, return an error
	return nil, fmt.Errorf("no temperature files found")
}

// hwMonTemperatureMetrics collects temperature metrics from /sys/class/hwmon/hwmon*/temp*_input
func hwMonTemperatureMetrics() ([]Metric, error) {

	var files []string
	var err error

	globPath := filepath.Join(sysFS, "class", "hwmon", "hwmon*", "temp*_input")

	if files, err = filepath.Glob(globPath); err != nil {
		return nil, err
	}

	if len(files) == 0 {
		// CentOS has an intermediate /device directory:
		// https://github.com/giampaolo/psutil/issues/971
		globPath = filepath.Join(sysFS, "class", "hwmon", "hwmon*", "device", "temp*_input")
		if files, err = filepath.Glob(globPath); err != nil {
			return nil, err
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no temperature metrics found")
	}

	// Collect temperature metrics
	metrics := make([]Metric, 0)

	for _, file := range files {
		var raw []byte

		var temperature float64

		// Get the base directory location
		directory := filepath.Dir(file)

		// Get the base filename prefix like temp1
		basename := strings.Split(filepath.Base(file), "_")[0]

		// Get the base path like <dir>/temp1
		basepath := filepath.Join(directory, basename)

		// Get the label of the temperature you are reading
		label := ""

		if raw, _ = os.ReadFile(basepath + "_label"); len(raw) != 0 {
			// Format the label from "Core 0" to "core_0"
			label = strings.Join(strings.Split(strings.TrimSpace(strings.ToLower(string(raw))), " "), "_")
		}

		// Get the name of the temperature you are reading
		if raw, err = os.ReadFile(filepath.Join(directory, "name")); err != nil {
			continue
		}

		name := strings.TrimSpace(string(raw))

		if label != "" {
			name = name + "_" + label
		}

		// Get the temperature reading
		if raw, err = os.ReadFile(file); err != nil {
			continue
		}

		if temperature, err = strconv.ParseFloat(strings.TrimSpace(string(raw)), 64); err != nil {
			continue
		}

		// Skip temperatures below 0, assume they are invalid
		if temperature <= 0 {
			continue
		}

		metric := Metric{
			Label:     Temperature,
			Timestamp: time.Now().Unix(),
			ID:        strings.TrimSpace(string(name)),
			Values: Values{
				TemperatureValues: &TemperatureValues{
					Temperature: temperature / hostTemperatureScale,
				},
			},
		}

		metrics = append(metrics, metric)
	}

	return metrics, nil
}

func thermalZoneTemperatureMetrics() ([]Metric, error) {

	globPath := filepath.Join(sysFS, "class", "thermal", "thermal_zone*")
	files, err := filepath.Glob(globPath)

	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no thermal zone metrics found")
	}

	metrics := make([]Metric, 0)

	for _, file := range files {
		// Get the name of the temperature you are reading
		name, err := os.ReadFile(filepath.Join(file, "type"))
		if err != nil {
			continue
		}
		// Get the temperature reading
		current, err := os.ReadFile(filepath.Join(file, "temp"))
		if err != nil {
			continue
		}

		temperature, err := strconv.ParseInt(strings.TrimSpace(string(current)), 10, 64)
		if err != nil {
			continue
		}

		metric := Metric{
			Label:     Temperature,
			Timestamp: time.Now().Unix(),
			ID:        strings.TrimSpace(string(name)),
			Values: Values{
				TemperatureValues: &TemperatureValues{
					Temperature: float64(temperature) / hostTemperatureScale,
				},
			},
		}

		metrics = append(metrics, metric)
	}

	return metrics, nil
}
