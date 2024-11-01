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
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"go.qbee.io/agent/app/container"
)

// DockerContainersBundle controls docker containers running in the system.
//
// Example payload:
//
//	{
//		"items": [
//		  {
//	     "name": "container-a",
//	     "image": "debian:stable",
//	     "docker_args": "-v /path/to/data-volume:/data --hostname my-hostname",
//	     "env_file": "/my-directory/my-envfile",
//	     "command": "echo 'hello world!'"
//		  }
//		],
//	 "registry_auths": [
//	   {
//	      "server": "gcr.io",
//	      "username": "user",
//	      "password": "seCre7"
//	   }
//	 ]
//	}
type DockerContainersBundle struct {
	Metadata

	// Containers to be running in the system.
	Containers []container.Container `json:"items"`

	// RegistryAuths contains credentials to private docker registries.
	RegistryAuths []container.RegistryAuth `json:"registry_auths"`
}

// Execute docker containers configuration bundle on the system.
func (d DockerContainersBundle) Execute(ctx context.Context, service *Service) error {
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		ReportError(ctx, nil, "Docker container configuration configured, but no docker executable found on system")
		return fmt.Errorf("docker not supported: %v", err)
	}

	// populate all registry credentials
	for _, auth := range d.RegistryAuths {
		auth.ContainerRuntime = container.DockerRuntimeType
		auth.Server = resolveParameters(ctx, auth.Server)
		auth.Username = resolveParameters(ctx, auth.Username)
		auth.Password = resolveParameters(ctx, auth.Password)

		if err = executeAuth(ctx, dockerBin, auth); err != nil {
			ReportError(ctx, err, "Unable to authenticate with %s repository.", auth.URL())
			return err
		}
	}

	cacheDirectory := filepath.Join(service.cacheDirectory, PodmanContainerDirectory)
	userCacheDirectory := filepath.Join(service.userCacheDirectory, PodmanContainerDirectory)

	for containerIndex, execContainer := range d.Containers {
		execContainer.ContainerRuntime = container.DockerRuntimeType
		execContainer.Name = resolveParameters(ctx, execContainer.Name)
		execContainer.Image = resolveParameters(ctx, execContainer.Image)
		execContainer.Args = resolveParameters(ctx, execContainer.Args)
		execContainer.EnvFile = resolveParameters(ctx, execContainer.EnvFile)
		execContainer.Command = resolveParameters(ctx, execContainer.Command)
		execContainer.PreCondition = resolveParameters(ctx, execContainer.PreCondition)

		// for containers with empty name, use its index
		if execContainer.Name == "" {
			execContainer.Name = fmt.Sprintf("%d", containerIndex)
		}

		execContainer.SetCacheDirectory(cacheDirectory, userCacheDirectory)

		if err = executeContainer(ctx, service, dockerBin, execContainer); err != nil {
			return err
		}
	}

	return nil
}
