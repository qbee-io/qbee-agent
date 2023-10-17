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

package inventory

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qbee-io/qbee-agent/app/utils"
)

// TypeDockerNetworks is the inventory type for Docker networks.
const TypeDockerNetworks Type = "docker_networks"

// DockerNetworks represents a list of Docker networks.
type DockerNetworks struct {
	Networks []DockerNetwork `json:"items"`
}

// DockerNetwork represents a single Docker network.
type DockerNetwork struct {
	// ID - network ID.
	ID string `json:"id"`

	// Name - network name.
	Name string `json:"name"`

	// Driver - network driver (e.g. "bridge").
	Driver string `json:"driver"`

	// CreatedAt - time when the network was created (e.g. "2022-11-14 12:11:16.974857017 +0100 CET").
	CreatedAt string `json:"created_at"`

	// Internal - 'true' if the network is internal, 'false' if not.
	Internal string `json:"internal"`
}

const dockerNetworksFormat = `{` +
	`"id":"{{.ID}}",` +
	`"name":"{{.Name}}",` +
	`"driver":"{{.Driver}}",` +
	`"created_at":"{{.CreatedAt}}",` +
	`"internal":"{{.Internal}}"` +
	`}`

// CollectDockerNetworksInventory returns populated DockerNetworks inventory based on current system status.
func CollectDockerNetworksInventory(ctx context.Context) (*DockerNetworks, error) {
	if !HasDocker() {
		return nil, nil
	}

	cmd := []string{"docker", "network", "ls", "--no-trunc", "--format", dockerNetworksFormat}

	networks := make([]DockerNetwork, 0)

	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		network := new(DockerNetwork)

		if err := json.Unmarshal([]byte(line), network); err != nil {
			return fmt.Errorf("error decoding docker network: %w", err)
		}

		networks = append(networks, *network)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error listing docker networks: %w", err)
	}

	dockerNetworks := &DockerNetworks{
		Networks: networks,
	}

	return dockerNetworks, nil
}
