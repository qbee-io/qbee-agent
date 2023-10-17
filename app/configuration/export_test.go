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
	"strings"

	"github.com/qbee-io/qbee-agent/app/utils/runner"
)

// ResetRebootAfterRun allows to reset internal rebootAfterRun flag from tests.
func (srv *Service) ResetRebootAfterRun() {
	srv.rebootAfterRun = false
}

// ExecuteTestConfigInDocker executes provided config inside a docker container and returns reports and logs.
func ExecuteTestConfigInDocker(r *runner.Runner, config CommittedConfig) ([]string, []string) {

	r.MustExec("mkdir", "-p", "/etc/qbee/ppkeys")
	r.MustExec("touch", "/etc/qbee/ppkeys/ca.cert")

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
