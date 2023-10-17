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

package configuration_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"qbee.io/platform/test/assert"
	"qbee.io/platform/test/runner"

	"github.com/qbee-io/qbee-agent/app/configuration"
)

func Test_DockerContainers_Auths(t *testing.T) {
	r := runner.New(t)

	r.MustExec("apt-get", "install", "-y", "docker-ce-cli")

	dockerBundle := configuration.DockerContainersBundle{
		RegistryAuths: []configuration.RegistryAuth{
			{
				Username: "qbeetester",
				Password: "dckr_pat_rGiJ1QOxQNeVeJHbonJyXfKaZsY",
			},
		},
	}

	// running it the first time creates the docker config file with credentials
	reports := executeDockerContainersBundle(r, dockerBundle)

	expectedReports := []string{
		"[INFO] Configured credentials for https://index.docker.io/v1/.",
	}

	assert.Equal(t, reports, expectedReports)

	// check that a correct docker config file is created
	output := r.MustExec("cat", "/root/.docker/config.json")
	dockerConfig := make(map[string]any)

	if err := json.Unmarshal(output, &dockerConfig); err != nil {
		t.Fatalf("error decoding docker config: %v", err)
	}

	expectedDockerConfig := map[string]any{
		"auths": map[string]any{
			"https://index.docker.io/v1/": map[string]any{
				"auth": "cWJlZXRlc3RlcjpkY2tyX3BhdF9yR2lKMVFPeFFOZVZlSkhib25KeVhmS2Fac1k=",
			},
		},
	}
	assert.Equal(t, dockerConfig, expectedDockerConfig)

	// running it the second time does nothing
	reports = executeDockerContainersBundle(r, dockerBundle)
	assert.Empty(t, reports)
}

func Test_DockerContainers_Container_Start(t *testing.T) {
	r := runner.New(t)

	r.MustExec("apt-get", "install", "-y", "docker-ce-cli")

	containerName := fmt.Sprintf("%s-%d", t.Name(), time.Now().Unix())

	dockerBundle := configuration.DockerContainersBundle{
		Containers: []configuration.DockerContainer{
			{
				Name:       containerName,
				Image:      runner.Debian,
				DockerArgs: "--rm",
				Command:    "sleep 5",
			},
		},
	}

	// running it the first time starts a docker container
	reports := executeDockerContainersBundle(r, dockerBundle)
	expectedReports := []string{
		"[INFO] Successfully started container for image debian:qbee.",
	}

	assert.Equal(t, reports, expectedReports)

	// check that there is a container running with the specified command
	output := r.MustExec("docker", "container", "ls", "--filter", "name="+containerName, "--format", "{{.Command}}")
	assert.Equal(t, string(output), fmt.Sprintf(`"%s"`, dockerBundle.Containers[0].Command))

	// running it the second time does nothing, since the correct container is already running
	reports = executeDockerContainersBundle(r, dockerBundle)
	assert.Empty(t, reports)
}

func Test_DockerContainers_Container_StartExited(t *testing.T) {
	r := runner.New(t)

	r.MustExec("apt-get", "install", "-y", "docker-ce-cli")

	containerName := fmt.Sprintf("%s-%d", t.Name(), time.Now().Unix())

	dockerBundle := configuration.DockerContainersBundle{
		Containers: []configuration.DockerContainer{
			{
				Name:    containerName,
				Image:   runner.Debian,
				Command: "echo 1",
			},
		},
	}

	// running it the first time starts a new docker container (which exits immediately)
	reports := executeDockerContainersBundle(r, dockerBundle)
	expectedReports := []string{
		"[INFO] Successfully started container for image debian:qbee.",
	}
	assert.Equal(t, reports, expectedReports)

	time.Sleep(time.Second)

	// running it the second time re-starts exited container
	dockerBundle = configuration.DockerContainersBundle{
		Containers: []configuration.DockerContainer{
			{
				Name:       containerName,
				Image:      runner.Debian,
				DockerArgs: "--rm",
				Command:    "sleep 5",
			},
		},
	}
	reports = executeDockerContainersBundle(r, dockerBundle)
	expectedReports = []string{
		"[WARN] Container exited for image debian:qbee.",
		"[INFO] Successfully restarted container for image debian:qbee.",
	}
	assert.Equal(t, reports, expectedReports)

	// check that there is a container running with the specified command
	output := r.MustExec("docker", "container", "ls", "--filter", "name="+containerName, "--format", "{{.Command}}")
	assert.Equal(t, string(output), fmt.Sprintf(`"%s"`, dockerBundle.Containers[0].Command))
}

func Test_DockerContainers_Container_RestartOnConfigChange(t *testing.T) {
	r := runner.New(t)

	r.MustExec("apt-get", "install", "-y", "docker-ce-cli")

	containerName := fmt.Sprintf("%s-%d", t.Name(), time.Now().Unix())

	dockerBundle := configuration.DockerContainersBundle{
		Containers: []configuration.DockerContainer{
			{
				Name:    containerName,
				Image:   runner.Debian,
				Command: "sleep 5",
			},
		},
	}

	// running it the first time starts a new docker container (which exits immediately)
	reports := executeDockerContainersBundle(r, dockerBundle)
	expectedReports := []string{
		"[INFO] Successfully started container for image debian:qbee.",
	}
	assert.Equal(t, reports, expectedReports)

	// running it the second time re-starts running container
	dockerBundle = configuration.DockerContainersBundle{
		Containers: []configuration.DockerContainer{
			{
				Name:       containerName,
				Image:      runner.Debian,
				DockerArgs: "--rm",
				Command:    "sleep 5",
			},
		},
	}
	reports = executeDockerContainersBundle(r, dockerBundle)
	expectedReports = []string{
		"[WARN] Container configuration update detected for image debian:qbee.",
		"[INFO] Successfully restarted container for image debian:qbee.",
	}
	assert.Equal(t, reports, expectedReports)

	// check that there is a container running with the specified command
	output := r.MustExec("docker", "container", "ls", "--filter", "name="+containerName, "--format", "{{.Command}}")
	assert.Equal(t, string(output), fmt.Sprintf(`"%s"`, dockerBundle.Containers[0].Command))
}

// executeDockerContainersBundle is a helper method to quickly execute docker containers bundle.
// On success, it returns a slice of produced reports.
func executeDockerContainersBundle(r *runner.Runner, bundle configuration.DockerContainersBundle) []string {
	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleDockerContainers},
		BundleData: configuration.BundleData{
			DockerContainers: &bundle,
		},
	}

	config.BundleData.DockerContainers.Enabled = true

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)

	return reports
}
