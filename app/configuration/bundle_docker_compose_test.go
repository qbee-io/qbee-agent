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
	"go.qbee.io/agent/app/container"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_Simple_Docker_Compose(t *testing.T) {

	r := runner.New(t)

	dockerComposeBundle := configuration.DockerComposeBundle{
		Projects: []container.Compose{
			{
				Name: "project-a",
				File: "file:///compose/compose-nobuild.yml",
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
		"[INFO] Successfully downloaded file file:///compose/compose-nobuild.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/compose.yml",
		"[INFO] Started compose project project-a",
	}

	assert.Equal(t, reports, expectedReports)

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)

	assert.Empty(t, reports)

	dockerComposeBundle = configuration.DockerComposeBundle{
		Projects: []container.Compose{
			{
				Name: "project-b",
				File: "file:///compose/compose-nobuild.yml",
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
		"[INFO] Removed unconfigured compose project project-a",
		"[INFO] Successfully downloaded file file:///compose/compose-nobuild.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-b/compose.yml",
		"[INFO] Started compose project project-b",
	}

	r.MustExec("docker", "compose", "-p", "project-b", "down", "--remove-orphans", "--volumes", "--timeout", "60", "--rmi", "all")
	assert.Equal(t, reports, expectedReports)

}

func Test_ComposeWithBuildContext(t *testing.T) {

	r := runner.New(t)

	dockerComposeBundle := configuration.DockerComposeBundle{
		Projects: []container.Compose{
			{
				Name:    "project-a",
				File:    "file:///compose/compose-build.yml",
				Context: "file:///compose/context.tar.gz",
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
		"[INFO] Successfully downloaded file file:///compose/compose-build.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/compose.yml",
		"[INFO] Successfully downloaded file file:///compose/context.tar.gz to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/_tmp/context.tar.gz",
		"[INFO] Started compose project project-a",
	}

	assert.Equal(t, reports, expectedReports)

	// try again, should not download the context again
	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)
	assert.Empty(t, reports)

	dockerComposeBundle = configuration.DockerComposeBundle{
		Projects: []container.Compose{
			{
				Name:    "project-a",
				File:    "file:///compose/compose-build.yml",
				Context: "file:///compose/context.tar.bz2",
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
		"[INFO] Successfully downloaded file file:///compose/context.tar.bz2 to /var/lib/qbee/app_workdir/cache/docker_compose/project-a/_tmp/context.tar.bz2",
		"[INFO] Started compose project project-a",
	}
	assert.Equal(t, reports, expectedReports)
	r.MustExec("docker", "compose", "-p", "project-b", "down", "--remove-orphans", "--volumes", "--timeout", "60", "--rmi", "all")
}
