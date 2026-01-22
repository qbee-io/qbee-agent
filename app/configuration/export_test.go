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

const unprivilegedUser = "qbee-agent"

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

	// setup runner for unprivileged user if needed
	if r.GetUnprivileged() {
		// set up access to docker socket for unprivileged user
		gidOutput := r.MustExec("stat", "-c", "%g", "/var/run/docker.sock")
		gid := strings.TrimSpace(string(gidOutput))
		r.MustExec("groupadd", "-fg", gid, "docker")
		r.MustExec("usermod", "-aG", gid, unprivilegedUser)
		etcDir := filepath.Join("/var/lib", unprivilegedUser, "etc")
		stateDir := filepath.Join("/var/lib", unprivilegedUser, "var")

		r.MustExec("mkdir", "-p", etcDir)
		r.CreateFile(filepath.Join(etcDir, "qbee-agent.json"), []byte(`{"privilege_elevation": true}`))
		r.MustExec("chown", "-R", unprivilegedUser+":"+unprivilegedUser, etcDir)

		suCmd := "qbee-agent -c " + etcDir + " -s " + stateDir + " config -r -f /app/config.json"
		cmd = append([]string{"su", "-s", "/bin/sh", unprivilegedUser, "-c"}, suCmd)
	}

	r.CreateJSON("/app/config.json", config)
	r.MustExec("chmod", "644", "/app/config.json")

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
