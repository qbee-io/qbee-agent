package configuration

import (
	"strings"

	"github.com/qbee-io/qbee-agent/app/test"
)

// ResetRebootAfterRun allows to reset internal rebootAfterRun flag from tests.
func (srv *Service) ResetRebootAfterRun() {
	srv.rebootAfterRun = false
}

// ExecuteTestConfigInDocker executes provided config inside a docker container and returns reports and logs.
func ExecuteTestConfigInDocker(r *test.DockerRunner, config CommittedConfig) ([]string, []string) {
	r.CreateJSON("config.json", config)

	output := r.MustExec("qbee-agent", "config", "-f", "config.json")

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
