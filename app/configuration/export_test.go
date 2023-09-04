package configuration

import (
	"strings"

	"qbee.io/platform/test/device"
)

// ResetRebootAfterRun allows to reset internal rebootAfterRun flag from tests.
func (srv *Service) ResetRebootAfterRun() {
	srv.rebootAfterRun = false
}

// ExecuteTestConfigInDocker executes provided config inside a docker container and returns reports and logs.
func ExecuteTestConfigInDocker(r *device.Runner, config CommittedConfig) ([]string, []string) {
	r.Bootstrap()

	r.CreateJSON("/app/config.json", config)

	return ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r", "-f", "/app/config.json"))
}

// ParseTestConfigExecuteOutput parses logs and reports out of the configuration-execute command output.
func ParseTestConfigExecuteOutput(output []byte) ([]string, []string) {
	if len(output) == 0 {
		return nil, nil
	}

	reports := make([]string, 0)
	logs := make([]string, 0)

	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, consolePrefixReport) {
			reports = append(reports, strings.TrimSpace(strings.TrimPrefix(line, consolePrefixReport)))
		} else {
			logs = append(logs, strings.TrimSpace(strings.TrimPrefix(line, consolePrefixLog)))
		}
	}

	return reports, logs
}
