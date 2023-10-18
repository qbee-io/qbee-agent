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

package inventory

// TypeProcesses is the inventory type for process information.
const TypeProcesses Type = "processes"

// Processes contains information about processes running on the system.
type Processes struct {
	Processes []Process `json:"items"`
}

// Process contains information about a running process.
type Process struct {
	// PID - process ID.
	PID int `json:"pid"`

	// User - owner of the process.
	User string `json:"user"`

	// Memory - memory usage in percent.
	Memory float64 `json:"mem"`

	// CPU - CPU usage in percent.
	CPU float64 `json:"cpu"`

	// Command - program command.
	Command string `json:"cmdline"`
}
