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

package configuration

import (
	"context"
	"fmt"
	"strings"

	"github.com/qbee-io/qbee-agent/app/inventory/linux"
	"github.com/qbee-io/qbee-agent/app/utils"
)

// ProcessWatchBundle ensures running process are running (or not).
//
// Example payload:
//
//	{
//	  "processes": [
//	   {
//	     "name": "presentProcess",
//	     "policy": "Present",
//	     "command": "start.sh"
//	   },
//	   {
//	     "name": "absentProcess",
//	     "policy": "Absent",
//	     "command": "stop.sh"
//	   }
//	 ]
//	}
type ProcessWatchBundle struct {
	Metadata

	Processes []ProcessWatcher `json:"processes"`
}

// Execute ensures that watched processes are in a correct state.
func (p ProcessWatchBundle) Execute(ctx context.Context, _ *Service) error {
	runningProcesses, err := linux.ListRunningProcessesNames()
	if err != nil {
		return fmt.Errorf("cannot list running processes: %w", err)
	}

	for _, processWatcher := range p.Processes {
		if err = processWatcher.execute(ctx, runningProcesses); err != nil {
			return err
		}
	}

	return nil
}

// ProcessPolicy defines expected state of a process.
type ProcessPolicy string

// Supported process policies.
const (
	ProcessPresent ProcessPolicy = "Present"
	ProcessAbsent  ProcessPolicy = "Absent"
)

// ProcessWatcher defines a watcher for a process.
type ProcessWatcher struct {
	// Name of the process to watch.
	Name string `json:"name"`

	// Policy for the process.
	Policy ProcessPolicy `json:"policy"`

	// Command to use to get the process in the expected state.
	// For:
	// - ProcessPresent it should be a start command,
	// - ProcessAbsent it should be a stop command.
	Command string `json:"command"`
}

// execute the watcher policy on the defined process.
func (w ProcessWatcher) execute(ctx context.Context, runningProcesses map[string]string) error {
	w.Name = resolveParameters(ctx, w.Name)

	processIsRunning := false

	for _, processName := range runningProcesses {
		if strings.Contains(processName, w.Name) {
			processIsRunning = true
		}
	}

	switch w.Policy {
	case ProcessPresent:
		if processIsRunning {
			return nil
		}

		ReportInfo(ctx, nil, "Restarting process %s using defined command as it was not running", w.Name)
	case ProcessAbsent:
		if !processIsRunning {
			return nil
		}

		ReportInfo(ctx, nil, "Stopping process %s using defined command as it was found running", w.Name)
	}

	cmd := []string{getShell(), "-c", resolveParameters(ctx, w.Command)}

	output, err := utils.RunCommand(ctx, cmd)
	if err != nil {
		ReportError(ctx, err, "Error running command for process %s", w.Name)
		return err
	}

	ReportInfo(ctx, output, "Successfully ran command for process %s", w.Name)

	return nil
}
