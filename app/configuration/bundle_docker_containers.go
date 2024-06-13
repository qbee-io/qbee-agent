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
	Containers []Container `json:"items"`

	// RegistryAuths contains credentials to private docker registries.
	RegistryAuths []RegistryAuth `json:"registry_auths"`
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
		auth.ContainerRuntime = dockerRuntimeType
		auth.Server = resolveParameters(ctx, auth.Server)
		auth.Username = resolveParameters(ctx, auth.Username)
		auth.Password = resolveParameters(ctx, auth.Password)

		if err = auth.execute(ctx, dockerBin); err != nil {
			ReportError(ctx, err, "Unable to authenticate with %s repository.", auth.URL())
			return err
		}
	}

	for containerIndex, container := range d.Containers {
		container.ContainerRuntime = dockerRuntimeType
		container.Name = resolveParameters(ctx, container.Name)
		container.Image = resolveParameters(ctx, container.Image)
		container.Args = resolveParameters(ctx, container.Args)
		container.EnvFile = resolveParameters(ctx, container.EnvFile)
		container.Command = resolveParameters(ctx, container.Command)
		container.PreCondition = resolveParameters(ctx, container.PreCondition)

		// for containers with empty name, use its index
		if container.Name == "" {
			container.Name = fmt.Sprintf("%d", containerIndex)
		}

		if err = container.execute(ctx, service, dockerBin); err != nil {
			return err
		}
	}

	return nil
}
