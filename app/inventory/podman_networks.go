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

package inventory

import (
	"context"
	"encoding/json"
	"fmt"

	"go.qbee.io/agent/app/utils"
)

// TypePodmanNetworks is the inventory type for Podman networks.
const TypePodmanNetworks Type = "podman_networks"

// PodmanNetworks represents a list of Podman networks.
type PodmanNetworks struct {
	Networks []PodmanNetwork `json:"items"`
}

// PodmanNetwork represents a single Podman network.
type PodmanNetwork struct {
	// ID - network ID.
	ID string `json:"id"`

	// Name - network name.
	Name string `json:"name"`
}

const podmanNetworksFormat = `{` +
	`"id":"{{.ID}}",` +
	`"name":"{{.Name}}"` +
	`}`

// CollectPodmanNetworksInventory returns populated PodmanNetworks inventory based on current system status.
func CollectPodmanNetworksInventory(ctx context.Context) (*PodmanNetworks, error) {
	if !HasPodman() {
		return nil, nil
	}

	cmd := []string{"podman", "network", "ls", "--no-trunc", "--format", podmanNetworksFormat}

	networks := make([]PodmanNetwork, 0)

	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		network := new(PodmanNetwork)

		if err := json.Unmarshal([]byte(line), network); err != nil {
			return fmt.Errorf("error decoding podman network: %w", err)
		}

		networks = append(networks, *network)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error listing podman networks: %w", err)
	}

	podmanNetworks := &PodmanNetworks{
		Networks: networks,
	}

	return podmanNetworks, nil
}
