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
	"testing"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

// yaml is a superset of json, so we can use json as a valid yaml
const composeFileContents = `
{
	"version": "3",
	"services": {
		"web": {
			"image": "nginx:alpine"
		},
		"redis": {
			"image": "redis:alpine"
		}
	}
}`

func Test_Simple_Docker_Compose(t *testing.T) {

	r := runner.New(t)

	r.CreateFile("/compose.yml", []byte(composeFileContents))

	dockerComposeBundle := configuration.DockerComposeBundle{
		Projects: []configuration.DockerCompose{
			{
				Name:        "project-a",
				ComposeFile: "file:///compose.yml",
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
		"[INFO] Successfully downloaded file file:///compose.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-a.yml",
		"[INFO] Started compose project project-a",
	}

	assert.Equal(t, reports, expectedReports)

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)

	assert.Empty(t, reports)

	dockerComposeBundle = configuration.DockerComposeBundle{
		Projects: []configuration.DockerCompose{
			{
				Name:        "project-b",
				ComposeFile: "file:///compose.yml",
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
		"[INFO] Successfully downloaded file file:///compose.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-b.yml",
		"[INFO] Started compose project project-b",
	}

	assert.Equal(t, reports, expectedReports)
	r.MustExec("docker", "compose", "-p", "project-b", "down", "--remove-orphans")
}
