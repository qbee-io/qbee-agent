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

	"go.qbee.io/agent/app/utils"
)

// TypeDockerVolumes is the inventory type for Docker volumes.
const TypeDockerVolumes Type = "docker_volumes"

// DockerVolumes represents a list of Docker volumes.
type DockerVolumes struct {
	Volumes []DockerVolume `json:"items"`
}

// DockerVolume represents a single Docker volume.
type DockerVolume struct {
	// Name - volume name.
	Name string `json:"name"`

	// Driver - volume driver (e.g. "local").
	Driver string `json:"driver"`
}

const dockerVolumesFormat = `{` +
	`"name":"{{.Name}}",` +
	`"driver":"{{.Driver}}"` +
	`}`

// CollectDockerVolumesInventory returns populated DockerVolumes inventory based on current system status.
func CollectDockerVolumesInventory(ctx context.Context) (*DockerVolumes, error) {
	if !HasDocker() {
		return nil, nil
	}

	cmd := []string{"docker", "volume", "ls", "--format", dockerVolumesFormat}

	volumes := make([]DockerVolume, 0)

	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		volume := new(DockerVolume)

		if err := json.Unmarshal([]byte(line), volume); err != nil {
			return fmt.Errorf("error decoding docker volume: %w", err)
		}

		volumes = append(volumes, *volume)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error listing docker volumes: %w", err)
	}

	dockerVolumes := &DockerVolumes{
		Volumes: volumes,
	}

	return dockerVolumes, nil
}
