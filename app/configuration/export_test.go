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
	"path/filepath"
	"slices"
	"strings"

	"go.qbee.io/agent/app/utils/runner"
)

// ResetRebootAfterRun allows to reset internal rebootAfterRun flag from tests.
func (srv *Service) ResetRebootAfterRun() {
	srv.rebootAfterRun = false
}

// ExecuteTestConfigInDocker executes provided config inside a docker container and returns reports and logs.
func ExecuteTestConfigInDocker(r *runner.Runner, config CommittedConfig) ([]string, []string) {
	// if settings bundle is not set, add it to the config to stop the agent from reporting back to the device hub
	if !slices.Contains(config.Bundles, BundleSettings) {
		config.Bundles = append(config.Bundles, BundleSettings)

		config.BundleData.Settings = SettingsBundle{
			Metadata:      Metadata{Enabled: true},
			EnableReports: false,
		}
	}

	cmd := []string{"qbee-agent", "config", "-r", "-f", "/app/config.json"}

	r.CreateJSON("/app/config.json", config)

	// setup runner for unprivileged user if needed
	if r.GetUnprivileged() {
		// set up access to docker socket for unprivileged user if it exists
		if gidOutput, err := r.Exec("stat", "-c", "%g", "/var/run/docker.sock"); err == nil {
			gid := strings.TrimSpace(string(gidOutput))
			r.MustExec("groupadd", "-fg", gid, "docker")
			r.MustExec("usermod", "-aG", gid, runner.UnprivilegedUser)
		}

		etcDir := filepath.Join(filepath.Dir(r.GetStateDirectory()), "etc")

		r.MustExec("mkdir", "-p", etcDir)
		r.CreateFile(filepath.Join(etcDir, "qbee-agent.json"), []byte(`{"elevation_command":["/usr/bin/sudo", "-n"]}`))
		r.MustExec("chown", "-R", runner.UnprivilegedUser+":"+runner.UnprivilegedUser, etcDir)

		suCmd := "qbee-agent -c " + etcDir + " -s " + r.GetStateDirectory() + " config -r -f /app/config.json"
		cmd = append([]string{"su", "-s", "/bin/sh", runner.UnprivilegedUser, "-c"}, suCmd)
		r.MustExec("chmod", "644", "/app/config.json")
	}

	return ParseTestConfigExecuteOutput(r.MustExec(cmd...))
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
