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
	"time"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_PodmanContainers_Container_Start(t *testing.T) {
	r := runner.NewPodmanRunner(t)

	containerName := fmt.Sprintf("%s-%d", t.Name(), time.Now().Unix())

	podmanBundle := configuration.PodmanContainerBundle{
		Containers: []configuration.Container{
			{
				Name:    containerName,
				Image:   "alpine:latest",
				Args:    "--rm",
				Command: "sleep 5",
			},
		},
	}

	// running it the first time starts a docker container
	reports := executePodmanContainersBundle(r, podmanBundle)
	expectedReports := []string{
		"[INFO] Successfully started container for image alpine:latest.",
	}

	assert.Equal(t, reports, expectedReports)

	// check that there is a container running with the specified command

	output := r.MustExec("podman", "container", "ls", "--filter", "name="+containerName, "--format", "{{.Command}}")
	assert.Equal(t, string(output), podmanBundle.Containers[0].Command)

	// running it the second time does nothing, since the correct container is already running
	reports = executePodmanContainersBundle(r, podmanBundle)
	assert.Empty(t, reports)
}

func Test_PodmanContainers_Container_Change(t *testing.T) {
	r := runner.NewPodmanRunner(t)

	containerName := fmt.Sprintf("%s-%d", t.Name(), time.Now().Unix())

	podmanBundle := configuration.PodmanContainerBundle{
		Containers: []configuration.Container{
			{
				Name:    containerName,
				Image:   "alpine:latest",
				Args:    "--rm",
				Command: "sleep 5",
			},
		},
	}

	// running it the first time starts a docker container
	reports := executePodmanContainersBundle(r, podmanBundle)
	expectedReports := []string{
		"[INFO] Successfully started container for image alpine:latest.",
	}

	assert.Equal(t, reports, expectedReports)

	podmanBundle.Containers[0].Args = "--rm -p 8080:80"

	expectedReports = []string{
		"[WARN] Container configuration update detected for image alpine:latest.",
		"[INFO] Successfully restarted container for image alpine:latest.",
	}

	reports = executePodmanContainersBundle(r, podmanBundle)
	assert.Equal(t, reports, expectedReports)
}

func Test_PodmanContainers_Container_StartExited(t *testing.T) {
	r := runner.NewPodmanRunner(t)

	containerName := fmt.Sprintf("%s-%d", t.Name(), time.Now().Unix())

	podmanBundle := configuration.PodmanContainerBundle{
		Containers: []configuration.Container{
			{
				Name:    containerName,
				Image:   "alpine:latest",
				Command: "echo 1",
			},
		},
	}

	// running it the first time starts a new docker container (which exits immediately)
	reports := executePodmanContainersBundle(r, podmanBundle)
	expectedReports := []string{
		"[INFO] Successfully started container for image alpine:latest.",
	}
	assert.Equal(t, reports, expectedReports)

	time.Sleep(time.Second)

	// running it the second time re-starts exited container
	podmanBundle = configuration.PodmanContainerBundle{
		Containers: []configuration.Container{
			{
				Name:    containerName,
				Image:   "alpine:latest",
				Args:    "--rm",
				Command: "sleep 5",
			},
		},
	}
	reports = executePodmanContainersBundle(r, podmanBundle)
	expectedReports = []string{
		"[WARN] Container exited for image alpine:latest.",
		"[INFO] Successfully restarted container for image alpine:latest.",
	}
	assert.Equal(t, reports, expectedReports)

	// check that there is a container running with the specified command
	output := r.MustExec("podman", "container", "ls", "--filter", "name="+containerName, "--format", "{{.Command}}")
	assert.Equal(t, string(output), podmanBundle.Containers[0].Command)
}

func Test_PodmanContainers_As_User_Container_Start(t *testing.T) {
	r := runner.NewPodmanRunner(t)

	userName := "podman"
	r.MustExec("useradd", "-m", userName)

	containerName := fmt.Sprintf("%s-%d", t.Name(), time.Now().Unix())

	podmanBundle := configuration.PodmanContainerBundle{
		Containers: []configuration.Container{
			{
				Name:     containerName,
				Image:    "alpine:latest",
				Args:     "--rm",
				Command:  "sleep 5",
				ExecUser: userName,
			},
		},
	}

	// running it the first time starts a docker container
	reports := executePodmanContainersBundle(r, podmanBundle)
	expectedReports := []string{
		"[INFO] Successfully started container for image alpine:latest.",
	}

	assert.Equal(t, reports, expectedReports)

	// running it the second time does nothing, since the correct container is already running
	reports = executePodmanContainersBundle(r, podmanBundle)
	assert.Empty(t, reports)
}

// executePodmanContainersBundle is a helper method to quickly execute podman containers bundle.
// On success, it returns a slice of produced reports.
func executePodmanContainersBundle(r *runner.Runner, bundle configuration.PodmanContainerBundle) []string {
	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundlePodmanContainers},
		BundleData: configuration.BundleData{
			PodmanContainers: &bundle,
		},
	}

	config.BundleData.PodmanContainers.Enabled = true

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)

	return reports
}
