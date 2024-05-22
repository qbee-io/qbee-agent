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
)

type PodmanContainerBundle struct {
	Metadata

	// Containers is a list of containers to be managed.
	Containers []Container `json:"items"`

	// RegistryAuths is a list of registry authentication credentials.
	RegistryAuths []RegistryAuth `json:"registry_auths"`
}

// Execute ensures that the specified containers are in the desired state.
func (p PodmanContainerBundle) Execute(ctx context.Context, service *Service) error {
	podmanBin, err := exec.LookPath("podman")
	if err != nil {
		ReportError(ctx, nil, "Podman container configuration configured, but no podman executable found on system")
		return fmt.Errorf("cannot find podman binary: %w", err)
	}

	// populate all registry credentials
	for _, auth := range p.RegistryAuths {
		auth.ContainerRuntime = podmanRuntimeType
		auth.Server = resolveParameters(ctx, auth.Server)
		auth.Username = resolveParameters(ctx, auth.Username)
		auth.Password = resolveParameters(ctx, auth.Password)

		if err = auth.execute(ctx, podmanBin); err != nil {
			ReportError(ctx, err, "Unable to authenticate with %s repository.", auth.URL())
			return err
		}
	}

	for containerIndex, container := range p.Containers {
		container.ContainerRuntime = podmanRuntimeType
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

		if err = container.execute(ctx, service, podmanBin); err != nil {
			return err
		}
	}

	return nil
}
