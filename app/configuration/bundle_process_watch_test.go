package configuration_test

import (
	"testing"

	"github.com/qbee-io/qbee-agent/app/configuration"
	"github.com/qbee-io/qbee-agent/app/test"
)

func Test_BundleProcessWatch_ProcessPresent_NotRunning(t *testing.T) {
	r := test.NewDockerRunner(t, test.Debian)

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

	test.Equal(t, reports, expectedReports)

	expectedLogs := []string{"starting testProcess"}

	test.Equal(t, logs, expectedLogs)
}

func Test_BundleProcessWatch_ProcessPresent_AlreadyRunning(t *testing.T) {
	r := test.NewDockerRunner(t, test.Debian)

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

	test.Empty(t, reports)
	test.Empty(t, logs)
}

func Test_BundleProcessWatch_ProcessAbsent_NotRunning(t *testing.T) {
	r := test.NewDockerRunner(t, test.Debian)

	reports, logs := executeProcessWatchBundle(r, []configuration.ProcessWatcher{{
		Name:    "testProcess",
		Policy:  configuration.ProcessAbsent,
		Command: "echo 'stopping testProcess'",
	}})

	test.Empty(t, reports)
	test.Empty(t, logs)
}

func Test_BundleProcessWatch_ProcessAbsent_Running(t *testing.T) {
	r := test.NewDockerRunner(t, test.Debian)

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

	test.Equal(t, reports, expectedReports)

	expectedLogs := []string{"stopping testProcess"}

	test.Equal(t, logs, expectedLogs)
}

func Test_BundleProcessWatch_CommandError(t *testing.T) {
	r := test.NewDockerRunner(t, test.Debian)

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

	test.Equal(t, reports, expectedReports)

	expectedLogs := []string{
		"error running command [/bin/bash -c invalidCommand]: exit status 127",
		"/bin/bash: line 1: invalidCommand: command not found",
	}

	test.Equal(t, logs, expectedLogs)
}

// executePackageManagementBundle is a helper method to quickly execute process watch bundle.
// On success, it returns a slice of produced reports.
func executeProcessWatchBundle(r *test.DockerRunner, processes []configuration.ProcessWatcher) ([]string, []string) {
	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleProcessWatch},
		BundleData: configuration.BundleData{
			ProcessWatch: configuration.ProcessWatchBundle{
				Metadata:  configuration.Metadata{Enabled: true},
				Processes: processes,
			},
		},
	}

	return configuration.ExecuteTestConfigInDocker(r, config)
}
