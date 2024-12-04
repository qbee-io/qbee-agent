// Copyright 2024 qbee.io
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
	"strings"
	"testing"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_Version_Parse(t *testing.T) {

	tests := []struct {
		name     string
		input    string
		expected string
		isCompat bool
	}{
		{
			name:     "v2.0.0",
			input:    "Docker Compose version v2.0.0",
			expected: "2.0.0",
			isCompat: true,
		},
		{
			name:     "2.0.0",
			input:    "Docker Compose version 2.0.0",
			expected: "2.0.0",
			isCompat: true,
		},
		{
			name:     "2.0.0",
			input:    "Docker Compose version 2.24.6+ds1-0ubuntu1~22.04.1",
			expected: "2.24.6",
			isCompat: true,
		},
		{
			name:     "1.1.1",
			input:    "Docker Compose version 1.1.1",
			expected: "1.1.1",
			isCompat: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := configuration.DockerComposeParseVersion(tt.input)
			assert.Empty(t, err)
			assert.Equal(t, version, tt.expected)
			assert.Equal(t, utils.IsNewerVersionOrEqual(version, configuration.DockerComposeMinimumVersion), tt.isCompat)
		})
	}
}

func Test_Simple_Docker_Compose(t *testing.T) {

	r := runner.New(t)

	dockerComposeBundle := configuration.DockerComposeBundle{
		Projects: []configuration.Compose{
			{
				Name: "project-a",
				File: "file:///docker-compose/compose-nobuild.yml",
			},
		},
	}

	dockerComposeBundle.Enabled = true

	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleDockerCompose},
		BundleData: configuration.BundleData{
			DockerCompose: &dockerComposeBundle,
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)
	expectedReports := []string{
		"[INFO] Successfully downloaded file file:///docker-compose/compose-nobuild.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/compose.yml",
		"[INFO] Started compose project project-a",
	}

	assert.Equal(t, reports, expectedReports)

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)

	assert.Empty(t, reports)

	dockerComposeBundle = configuration.DockerComposeBundle{
		Projects: []configuration.Compose{
			{
				Name: "project-b",
				File: "file:///docker-compose/compose-nobuild.yml",
			},
		},
		Clean: true,
	}

	dockerComposeBundle.Enabled = true

	config = configuration.CommittedConfig{
		Bundles: []string{configuration.BundleDockerCompose},
		BundleData: configuration.BundleData{
			DockerCompose: &dockerComposeBundle,
		},
	}

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)
	expectedReports = []string{
		"[INFO] Successfully downloaded file file:///docker-compose/compose-nobuild.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-b/compose.yml",
		"[INFO] Started compose project project-b",
	}

	assert.Equal(t, reports, expectedReports)
	r.MustExec("docker", "compose", "-p", "project-b", "down", "--remove-orphans", "--volumes", "--timeout", "60", "--rmi", "all")
}

func Test_ComposeWithParams(t *testing.T) {

	r := runner.New(t)

	dockerComposeBundle := configuration.DockerComposeBundle{
		Projects: []configuration.Compose{
			{
				Name: "project-a",
				File: "file:///docker-compose/compose-nobuild.yml.template",
				Parameters: []configuration.TemplateParameter{
					{
						Key:   "param1",
						Value: "value1",
					},
				},
			},
		},
	}

	dockerComposeBundle.Enabled = true

	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleDockerCompose},
		BundleData: configuration.BundleData{
			DockerCompose: &dockerComposeBundle,
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)
	expectedReports := []string{
		"[INFO] Successfully rendered template file file:///docker-compose/compose-nobuild.yml.template to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/compose.yml",
		"[INFO] Started compose project project-a",
	}

	assert.Equal(t, reports, expectedReports)

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)

	assert.Empty(t, reports)
	r.MustExec("docker", "compose", "-p", "project-a", "down", "--remove-orphans", "--volumes", "--timeout", "60", "--rmi", "all")
}

func Test_ComposeWithBuildContext(t *testing.T) {

	r := runner.New(t)

	dockerComposeBundle := configuration.DockerComposeBundle{
		Projects: []configuration.Compose{
			{
				Name:       "project-a",
				File:       "file:///docker-compose/compose-build.yml",
				Context:    "file:///docker-compose/context.tar.gz",
				UseContext: true,
			},
		},
	}

	dockerComposeBundle.Enabled = true

	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleDockerCompose},
		BundleData: configuration.BundleData{
			DockerCompose: &dockerComposeBundle,
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)
	expectedReports := []string{
		"[INFO] Successfully downloaded file file:///docker-compose/compose-build.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/compose.yml",
		"[INFO] Successfully downloaded file file:///docker-compose/context.tar.gz to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/_tmp/context.tar.gz",
		"[INFO] Started compose project project-a",
	}

	assert.Equal(t, reports, expectedReports)

	// try again, should not download the context again
	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)
	assert.Empty(t, reports)

	dockerComposeBundle = configuration.DockerComposeBundle{
		Projects: []configuration.Compose{
			{
				Name:       "project-a",
				File:       "file:///docker-compose/compose-build.yml",
				Context:    "file:///docker-compose/context.tar.bz2",
				UseContext: true,
			},
		},
	}

	dockerComposeBundle.Enabled = true

	config = configuration.CommittedConfig{
		Bundles: []string{configuration.BundleDockerCompose},
		BundleData: configuration.BundleData{
			DockerCompose: &dockerComposeBundle,
		},
	}

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)

	expectedReports = []string{
		"[INFO] Successfully downloaded file file:///docker-compose/context.tar.bz2 to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/_tmp/context.tar.bz2",
		"[INFO] Started compose project project-a",
	}
	assert.Equal(t, reports, expectedReports)
	r.MustExec("docker", "compose", "-p", "project-b", "down", "--remove-orphans", "--volumes", "--timeout", "60", "--rmi", "all")
}

func Test_ComposeWithPreCondition(t *testing.T) {

	r := runner.New(t)

	dockerComposeBundle := configuration.DockerComposeBundle{
		Projects: []configuration.Compose{
			{
				Name:         "project-a",
				File:         "file:///docker-compose/compose-nobuild.yml",
				PreCondition: "/bin/false",
			},
		},
	}

	dockerComposeBundle.Enabled = true

	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleDockerCompose},
		BundleData: configuration.BundleData{
			DockerCompose: &dockerComposeBundle,
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)

	assert.Empty(t, reports)
}

func Test_ComposeWithSkipRestart(t *testing.T) {

	r := runner.New(t)

	dockerComposeBundle := configuration.DockerComposeBundle{
		Projects: []configuration.Compose{
			{
				Name:        "project-a",
				File:        "file:///docker-compose/compose-nobuild.yml",
				SkipRestart: true,
			},
		},
	}

	dockerComposeBundle.Enabled = true

	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleDockerCompose},
		BundleData: configuration.BundleData{
			DockerCompose: &dockerComposeBundle,
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)
	expectedReports := []string{
		"[INFO] Successfully downloaded file file:///docker-compose/compose-nobuild.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/compose.yml",
		"[INFO] Started compose project project-a",
	}
	assert.Equal(t, reports, expectedReports)

	r.MustExec("docker", "kill", "project-a-web-1")

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)

	assert.Empty(t, reports)

	dockerComposeBundle.Projects[0].SkipRestart = false

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)
	expectedReports = []string{
		"[WARN] One or more containers in exited state for project project-a. Restart scehduled",
		"[INFO] Started compose project project-a",
	}

	assert.Equal(t, reports, expectedReports)
	r.MustExec("docker", "compose", "-p", "project-a", "down", "--remove-orphans", "--volumes", "--timeout", "60", "--rmi", "all")

}

func Test_ComposeWithClean(t *testing.T) {

	r := runner.New(t)

	dockerComposeBundle := configuration.DockerComposeBundle{
		Projects: []configuration.Compose{
			{
				Name: "project-a",
				File: "file:///docker-compose/compose-nobuild.yml",
			},
		},
	}

	dockerComposeBundle.Enabled = true

	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleDockerCompose},
		BundleData: configuration.BundleData{
			DockerCompose: &dockerComposeBundle,
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)
	expectedReports := []string{
		"[INFO] Successfully downloaded file file:///docker-compose/compose-nobuild.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/compose.yml",
		"[INFO] Started compose project project-a",
	}
	assert.Equal(t, reports, expectedReports)

	dockerComposeBundle.Clean = true
	dockerComposeBundle.Projects = []configuration.Compose{}

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)
	assert.Empty(t, reports)

	output := r.MustExec("docker", "compose", "ls", "--all")

	exists := strings.Contains(string(output), "project-a")

	assert.False(t, exists)
}
