package configuration_test

import (
	"testing"

	"qbee.io/platform/test/assert"
	"qbee.io/platform/test/device"

	"github.com/qbee-io/qbee-agent/app/configuration"
)

func Test_BundleProcessWatch_ProcessPresent_NotRunning(t *testing.T) {
	r := device.New(t)

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
	r := device.New(t)

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
	r := device.New(t)

	reports, logs := executeProcessWatchBundle(r, []configuration.ProcessWatcher{{
		Name:    "testProcess",
		Policy:  configuration.ProcessAbsent,
		Command: "echo 'stopping testProcess'",
	}})

	assert.Empty(t, reports)
	assert.Empty(t, logs)
}

func Test_BundleProcessWatch_ProcessAbsent_Running(t *testing.T) {
	r := device.New(t)

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
	r := device.New(t)

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
func executeProcessWatchBundle(r *device.Runner, processes []configuration.ProcessWatcher) ([]string, []string) {
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
