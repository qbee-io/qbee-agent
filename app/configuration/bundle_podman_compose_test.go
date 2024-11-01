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
	"fmt"
	"testing"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/container"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_Simple_Podman_Compose(t *testing.T) {

	r := runner.NewPodmanRunner(t)

	podmanComposeBundle := configuration.PodmanComposeBundle{
		Projects: []container.Compose{
			{
				Name: "project-a",
				File: "file:///compose/compose-nobuild.yml",
			},
		},
	}

	podmanComposeBundle.Enabled = true

	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundlePodmanCompose},
		BundleData: configuration.BundleData{
			PodmanCompose: &podmanComposeBundle,
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)
	expectedReports := []string{
		"[INFO] Successfully downloaded file file:///compose/compose-nobuild.yml to /var/lib/qbee/app_workdir/cache/podman_compose/project-a/compose.yml",
		"[INFO] Started compose project project-a",
	}

	assert.Equal(t, reports, expectedReports)

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)

	assert.Empty(t, reports)

	output := r.MustExec("podman", "ps")

	fmt.Println(string(output))

	podmanComposeBundle = configuration.PodmanComposeBundle{
		Projects: []container.Compose{
			{
				Name: "project-b",
				File: "file:///compose/compose-nobuild.yml",
			},
		},
		Clean: true,
	}

	podmanComposeBundle.Enabled = true

	config = configuration.CommittedConfig{
		Bundles: []string{configuration.BundlePodmanCompose},
		BundleData: configuration.BundleData{
			PodmanCompose: &podmanComposeBundle,
		},
	}

	reports, _ = configuration.ExecuteTestConfigInDocker(r, config)
	expectedReports = []string{
		"[INFO] Removed unconfigured compose project project-a",
		"[INFO] Successfully downloaded file file:///compose/compose-nobuild.yml to /var/lib/qbee/app_workdir/cache/docker_compose/project-b/compose.yml",
		"[INFO] Started compose project project-b",
	}

	r.MustExec("podman-compose", "-p", "project-b", "down", "--volumes", "--timeout", "60")
	assert.Equal(t, reports, expectedReports)

}
