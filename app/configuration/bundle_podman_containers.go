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

package configuration

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"go.qbee.io/agent/app/container"
)

// PodmanContainerBundle controls docker containers running in the system.
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
type PodmanContainerBundle struct {
	Metadata

	// Containers is a list of containers to be managed.
	Containers []container.Container `json:"items"`

	// RegistryAuths is a list of registry authentication credentials.
	RegistryAuths []container.RegistryAuth `json:"registry_auths"`
}

// Execute ensures that the specified containers are in the desired state.
func (p PodmanContainerBundle) Execute(ctx context.Context, service *Service) error {
	podmanBin, err := exec.LookPath("podman")
	if err != nil {
		ReportError(ctx, nil, "Podman container configuration configured, but no podman executable found on system")
		return fmt.Errorf("cannot find podman binary: %w", err)
	}

	// populate all registry credentials
	for _, execAuth := range p.RegistryAuths {
		execAuth.ContainerRuntime = container.PodmanRuntimeType
		execAuth.Server = resolveParameters(ctx, execAuth.Server)
		execAuth.Username = resolveParameters(ctx, execAuth.Username)
		execAuth.Password = resolveParameters(ctx, execAuth.Password)

		if err = executeAuth(ctx, podmanBin, execAuth); err != nil {
			ReportError(ctx, err, "Unable to authenticate with %s repository.", execAuth.URL())
			return err
		}
	}

	// execute all containers
	cacheDirectory := filepath.Join(service.cacheDirectory, PodmanContainerDirectory)
	userCacheDirectory := filepath.Join(service.userCacheDirectory, PodmanContainerDirectory)

	for containerIndex, execContainer := range p.Containers {
		execContainer.ContainerRuntime = container.PodmanRuntimeType
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

		if err = executeContainer(ctx, service, podmanBin, execContainer); err != nil {
			return err
		}
	}

	return nil
}
