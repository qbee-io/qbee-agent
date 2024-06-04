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

// TypePodmanVolumes is the inventory type for Podman volumes.
const TypePodmanVolumes Type = "podman_volumes"

// PodmanVolumes represents a list of Podman volumes.
type PodmanVolumes struct {
	Volumes []PodmanVolume `json:"items"`
}

// PodmanVolume represents a single Podman volume.
type PodmanVolume struct {
	// Name - volume name.
	Name string `json:"name"`

	// Driver - volume driver (e.g. "local").
	Driver string `json:"driver"`
}

const podmanVolumesFormat = `{` +
	`"name":"{{.Name}}",` +
	`"driver":"{{.Driver}}"` +
	`}`

// CollectPodmanVolumesInventory returns populated PodmanVolumes inventory based on current system status.
func CollectPodmanVolumesInventory(ctx context.Context) (*PodmanVolumes, error) {
	if !HasPodman() {
		return nil, nil
	}

	cmd := []string{"podman", "volume", "ls", "--format", podmanVolumesFormat}

	volumes := make([]PodmanVolume, 0)

	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		volume := new(PodmanVolume)

		if err := json.Unmarshal([]byte(line), volume); err != nil {
			return fmt.Errorf("error decoding podman volume: %w", err)
		}

		volumes = append(volumes, *volume)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error listing podman volumes: %w", err)
	}

	podmanVolumes := &PodmanVolumes{
		Volumes: volumes,
	}

	return podmanVolumes, nil
}
