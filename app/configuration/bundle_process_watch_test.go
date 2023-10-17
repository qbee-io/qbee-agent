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

package configuration_test

import (
	"testing"

	"github.com/qbee-io/qbee-agent/app/configuration"
	"github.com/qbee-io/qbee-agent/app/utils/assert"
	"github.com/qbee-io/qbee-agent/app/utils/runner"
)

func Test_BundleProcessWatch_ProcessPresent_NotRunning(t *testing.T) {
	r := runner.New(t)

	// create a test process script
	script := []byte("#!/bin/sh\nsleep 10")
	scriptPath := "/usr/bin/testProcess"
	r.CreateFile(scriptPath, script)
	r.MustExec("chmod", "+x", scriptPath)

	// running the bundle should launch the test process
	reports, logs := executeProcessWatchBundle(r, []configuration.ProcessWatcher{{
		Name:    "testProcess",
		Policy:  configuration.ProcessPresent,
		Command: "echo 'starting testProcess'",
	}})

	expectedReports := []string{
		"[INFO] Restarting process testProcess using defined command as it was not running",
		"[INFO] Successfully ran command for process testProcess",
	}

	assert.Equal(t, reports, expectedReports)

	expectedLogs := []string{"starting testProcess"}

	assert.Equal(t, logs, expectedLogs)
}

func Test_BundleProcessWatch_ProcessPresent_AlreadyRunning(t *testing.T) {
	r := runner.New(t)

	// create a test process script
	script := []byte("#!/bin/sh\nsleep 10")
	scriptPath := "/usr/bin/testProcess"
	r.CreateFile(scriptPath, script)
	r.MustExec("chmod", "+x", scriptPath)
	r.MustExec("sh", "-c", "/usr/bin/testProcess </dev/null &>/dev/null &")

	// running the bundle should do nothing
	reports, logs := executeProcessWatchBundle(r, []configuration.ProcessWatcher{{
		Name:    "testProcess",
		Policy:  configuration.ProcessPresent,
		Command: "echo 'starting testProcess'",
	}})

	assert.Empty(t, reports)
	assert.Empty(t, logs)
}

func Test_BundleProcessWatch_ProcessAbsent_NotRunning(t *testing.T) {
	r := runner.New(t)

	reports, logs := executeProcessWatchBundle(r, []configuration.ProcessWatcher{{
		Name:    "testProcess",
		Policy:  configuration.ProcessAbsent,
		Command: "echo 'stopping testProcess'",
	}})

	assert.Empty(t, reports)
	assert.Empty(t, logs)
}

func Test_BundleProcessWatch_ProcessAbsent_Running(t *testing.T) {
	r := runner.New(t)

	// create a test process script
	script := []byte("#!/bin/sh\nsleep 10")
	scriptPath := "/usr/bin/testProcess"
	r.CreateFile(scriptPath, script)
	r.MustExec("chmod", "+x", scriptPath)
	r.MustExec("sh", "-c", "/usr/bin/testProcess </dev/null &>/dev/null &")

	// running the bundle should do nothing
	reports, logs := executeProcessWatchBundle(r, []configuration.ProcessWatcher{{
		Name:    "testProcess",
		Policy:  configuration.ProcessAbsent,
		Command: "echo 'stopping testProcess'",
	}})

	expectedReports := []string{
		"[INFO] Stopping process testProcess using defined command as it was found running",
		"[INFO] Successfully ran command for process testProcess",
	}

	assert.Equal(t, reports, expectedReports)

	expectedLogs := []string{"stopping testProcess"}

	assert.Equal(t, logs, expectedLogs)
}

func Test_BundleProcessWatch_CommandError(t *testing.T) {
	r := runner.New(t)

	// running the bundle should cause an error
	reports, logs := executeProcessWatchBundle(r, []configuration.ProcessWatcher{{
		Name:    "testProcess",
		Policy:  configuration.ProcessPresent,
		Command: "invalidCommand",
	}})

	expectedReports := []string{
		"[INFO] Restarting process testProcess using defined command as it was not running",
		"[ERR] Error running command for process testProcess",
	}

	assert.Equal(t, reports, expectedReports)

	expectedLogs := []string{
		"error running command [/usr/bin/bash -c invalidCommand]: exit status 127",
		"/usr/bin/bash: line 1: invalidCommand: command not found",
	}

	assert.Equal(t, logs, expectedLogs)
}

// executePackageManagementBundle is a helper method to quickly execute process watch bundle.
// On success, it returns a slice of produced reports.
func executeProcessWatchBundle(r *runner.Runner, processes []configuration.ProcessWatcher) ([]string, []string) {
	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleProcessWatch},
		BundleData: configuration.BundleData{
			ProcessWatch: &configuration.ProcessWatchBundle{
				Metadata:  configuration.Metadata{Enabled: true},
				Processes: processes,
			},
		},
	}

	return configuration.ExecuteTestConfigInDocker(r, config)
}
