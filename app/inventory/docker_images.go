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

// TypeDockerImages is the inventory type for Docker images.
const TypeDockerImages Type = "docker_images"

// DockerImages represents a list of Docker images.
type DockerImages struct {
	Images []DockerImage `json:"items"`
}

// DockerImage represents a single Docker image.
type DockerImage struct {
	// ID - image ID.
	ID string `json:"id"`

	// Repository - image repository.
	Repository string `json:"repository"`

	// Tag - image tag.
	Tag string `json:"tag"`

	// CreatedAt - when the image was created (e.g. "2022-11-12 07:27:47 +0100 CET").
	CreatedAt string `json:"created_at"`

	// Size - image disk size.
	Size string `json:"size"`
}

const dockerImagesFormat = `{"id":"{{.ID}}","repository":"{{.Repository}}","tag":"{{.Tag}}",` +
	`"created_at":"{{.CreatedAt}}","size":"{{.Size}}"}`

// CollectDockerImagesInventory returns populated DockerImages inventory based on current system status.
func CollectDockerImagesInventory(ctx context.Context) (*DockerImages, error) {
	if !HasDocker() {
		return nil, nil
	}

	cmd := []string{"docker", "image", "ls", "--no-trunc", "--all", "--format", dockerImagesFormat}

	images := make([]DockerImage, 0)

	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		image := new(DockerImage)

		if err := json.Unmarshal([]byte(line), image); err != nil {
			return fmt.Errorf("error decoding docker image: %w", err)
		}

		images = append(images, *image)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error listing docker images: %w", err)
	}

	dockerImages := &DockerImages{
		Images: images,
	}

	return dockerImages, nil
}
