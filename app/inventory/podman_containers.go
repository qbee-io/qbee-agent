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

// TypePodmanContainers is the inventory type for Podman containers.
const TypePodmanContainers Type = "podman_containers"

// PodmanContainers represents a list of Podman containers.
type PodmanContainers struct {
	Containers []PodmanContainer `json:"items"`
}

// PodmanContainer represents a single Podman container.
type PodmanContainer struct {
	// ID - container ID.
	ID string `json:"id"`

	// Names - container names.
	Names string `json:"names"`

	// Image - image used to start the container.
	Image string `json:"image"`

	// Command - command running inside the container.
	Command string `json:"command"`

	// CreatedAt - container creation date/time (e.g. "2022-11-14 12:12:10 +0100 CET").
	CreatedAt string `json:"created_at"`

	// Status - container status (e.g. "Up 30 minutes").
	Status string `json:"status"`

	// Ports - network ports exposed by the container.
	// Examples:
	// - single port: 80/tcp
	// - range of ports: 80-81/tcp
	// - discontiguous ports: 80-81/tcp, 83/tcp
	Ports string `json:"ports"`

	// Size - container disk size.
	Size string `json:"size"`

	// Mounts - names of the volumes mounted in this container.
	Mounts string `json:"mounts"`

	// Networks - names of the networks attached to this container.
	Networks string `json:"networks"`
}

const PodmanContainersFormat = `{"id":"{{.ID}}","names":"{{.Names}}","image":"{{.Image}}","command":"{{.Command}}",` +
	`"created_at":"{{.CreatedAt}}","status":"{{.Status}}","ports":"{{.Ports}}","size":"{{.Size}}",` +
	`"mounts":"{{.Mounts}}", "networks":"{{.Networks}}"}`

// CollectPodmanContainersInventory returns populated PodmanContainers inventory based on current system status.
func CollectPodmanContainersInventory(ctx context.Context) (*PodmanContainers, error) {
	if !HasPodman() {
		return nil, nil
	}

	cmd := []string{"podman", "container", "ls", "--no-trunc", "--all", "--size", "--format", PodmanContainersFormat}

	containers := make([]PodmanContainer, 0)

	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		container := new(PodmanContainer)

		if err := json.Unmarshal([]byte(line), container); err != nil {
			return fmt.Errorf("error decoding podman container: %w", err)
		}

		containers = append(containers, *container)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error listing podman containers: %w", err)
	}

	PodmanContainers := &PodmanContainers{
		Containers: containers,
	}

	return PodmanContainers, nil
}
