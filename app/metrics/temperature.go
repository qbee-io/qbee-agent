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

	"go.qbee.io/agent/app/inventory/linux"
)

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

// Balena uses the following, only
// Math.round(tempInfo.main)

const hostTemperatureScale = 1000.0

// CollectTemperature collects temperature metrics from from /sys/class/[hwmon|thermal]
func CollectTemperature() ([]Metric, error) {

	var cpuTemps *cpuTemperatures
	var errString string

	// Attempt to collect temperature metrics from hwmon
	if err := cpuTemps.hwMonTemperatureMetrics(); err != nil {
		errString += err.Error()
	}

	// Attempt to collect temperature metrics from thermal zone
	if err := cpuTemps.thermalZoneTemperatureMetrics(); err != nil {
		errString += err.Error()
	}

	if cpuTemps.main > 0 {
		return []Metric{
			{
				Label:     Temperature,
				Timestamp: time.Now().Unix(),
				ID:        "cpu_temp",
				Values: Values{
					TemperatureValues: &TemperatureValues{
						Temperature: cpuTemps.main,
					},
				},
			},
		}, nil
	}
	return nil, fmt.Errorf("no temperature metrics found: %s", errString)
}

// hwMonTemperatureMetrics collects temperature metrics from /sys/class/hwmon/hwmon*/temp*_input
func (c *cpuTemperatures) hwMonTemperatureMetrics() error {

	files, err := getHwMonFiles()
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no hwmon temperature metrics found")
	}

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

		// Some boards have a cpu_thermal zone
		if strings.HasPrefix(label, "cpu_thermal") {
			temperature, err = parseTemperatureFile(file)
			if err != nil {
				continue
			}
			c.main = temperature
		}

		// Capture all core temperatures
		if strings.HasPrefix(label, "core") {
			temperature, err = parseTemperatureFile(file)
			if err != nil {
				continue
			}
			c.cores = append(c.cores, temperature)
		}

		// Capture all socket temperatures
		if strings.Contains(label, "package") || strings.Contains(label, "physical") || label == "tccd1" {
			temperature, err = parseTemperatureFile(file)
			if err != nil {
				continue
			}
			c.socket = append(c.socket, temperature)
		}
	}

	// calculate the average core temperature
	if len(c.cores) > 0 && c.main == 0 {
		c.main = 0
		for _, core := range c.cores {
			c.main += core
		}
		c.main = c.main / float64(len(c.cores))
	}

	return nil
}

// thermalZoneTemperatureMetrics collects temperature metrics from /sys/class/thermal/thermal_zone*/temp
func (c *cpuTemperatures) thermalZoneTemperatureMetrics() error {

	files, err := getThermalZoneFiles()
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no thermal zone metrics found")
	}

	for _, file := range files {
		// Get the name of the temperature you are reading
		rawName, err := os.ReadFile(filepath.Join(file, "type"))
		if err != nil {
			continue
		}

		name := strings.ToLower(strings.TrimSpace(string(rawName)))

		// Some boards have acpi_thermal zones
		if strings.HasPrefix(name, "acpi") {
			acpiTemp, err := parseTemperatureFile(filepath.Join(file, "temp"))
			if err != nil {
				continue
			}
			c.socket = append(c.socket, acpiTemp)
		}

		// Some boards have a pch_thermal zone
		if strings.HasPrefix(name, "pch") {
			chipsetTemp, err := parseTemperatureFile(filepath.Join(file, "temp"))
			if err != nil {
				continue
			}
			c.chipset = chipsetTemp
		}
		// ARM based boards have cpu-thermal zones
		if strings.HasPrefix(name, "cpu-thermal") {
			cpuTemp, err := parseTemperatureFile(filepath.Join(file, "temp"))
			if err != nil {
				continue
			}
			c.main = cpuTemp
		}
	}
	return nil
}

// parseTemperatureFile reads a temperature file and returns the temperature in degrees Celsius
func parseTemperatureFile(file string) (float64, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}

	temperature, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
	if err != nil {
		return 0, err
	}

	return temperature / hostTemperatureScale, nil
}

// getThermalZoneFiles returns a list of temperature files in /sys/class/thermal/thermal_zone*/temp
func getThermalZoneFiles() ([]string, error) {
	globPath := filepath.Join(linux.SysFS, "class", "thermal", "thermal_zone*")
	files, err := filepath.Glob(globPath)
	if err != nil {
		return nil, err
	}

	return files, nil
}

// getHwMonFiles returns a list of temperature files in /sys/class/hwmon/hwmon*/temp*_input
func getHwMonFiles() ([]string, error) {
	globPath := filepath.Join(linux.SysFS, "class", "hwmon", "hwmon*", "temp*_input")
	files, err := filepath.Glob(globPath)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		// CentOS has an intermediate /device directory:
		// https://github.com/giampaolo/psutil/issues/971
		globPath = filepath.Join(linux.SysFS, "class", "hwmon", "hwmon*", "device", "temp*_input")
		if files, err = filepath.Glob(globPath); err != nil {
			return nil, err
		}
	}

	return files, nil
}
