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
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.qbee.io/agent/app/inventory/linux"
)

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
	filePath := filepath.Join(linux.ProcFS, "stat")

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filePath, err)
	}

	defer file.Close()

	// we don't need to read the whole file, we only care about the first line
	buf := make([]byte, 512)
	if _, err = file.Read(buf); err != nil {
		return nil, fmt.Errorf("error reading contents of %s: %w", filePath, err)
	}

	firstLine := string(buf[0:bytes.Index(buf, []byte("\n"))])
	lineFields := strings.Fields(firstLine)

	fields := []string{"user", "nice", "system", "idle", "iowait", "irq"}
	fieldValues := make(map[string]uint64)

	for i, field := range fields {
		if fieldValues[field], err = strconv.ParseUint(lineFields[i+1], 10, 64); err != nil {
			return nil, fmt.Errorf("error parsing %s field in %s: %w", field, filePath, err)
		}
	}

	return &CPUValues{
		User:   float64(fieldValues["user"]),
		Nice:   float64(fieldValues["nice"]),
		System: float64(fieldValues["system"]),
		Idle:   float64(fieldValues["idle"]),
		IOWait: float64(fieldValues["iowait"]),
		IRQ:    float64(fieldValues["irq"]),
	}, nil
}

// Delta returns the delta between two CPUValues.
func (c *CPUValues) Delta(previous *CPUValues) (*CPUValues, error) {
	elapsed := c.totalTime() - previous.totalTime()
	if elapsed <= 0 {
		return nil, fmt.Errorf("elapsed time is <= 0: %f", elapsed)
	}

	user := 100 * (c.User - previous.User) / elapsed
	nice := 100 * (c.Nice - previous.Nice) / elapsed
	system := 100 * (c.System - previous.System) / elapsed
	idle := 100 * (c.Idle - previous.Idle) / elapsed
	iowait := 100 * (c.IOWait - previous.IOWait) / elapsed
	irq := 100 * (c.IRQ - previous.IRQ) / elapsed

	return &CPUValues{
		User:   math.Round(user*100) / 100,
		Nice:   math.Round(nice*100) / 100,
		System: math.Round(system*100) / 100,
		Idle:   math.Round(idle*100) / 100,
		IOWait: math.Round(iowait*100) / 100,
		IRQ:    math.Round(irq*100) / 100,
	}, nil
}

func (c *CPUValues) totalTime() float64 {
	return c.User + c.Nice + c.System + c.Idle + c.IOWait + c.IRQ
}
