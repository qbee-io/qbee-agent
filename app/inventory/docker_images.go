package inventory

import (
	"encoding/json"
	"fmt"

	"github.com/qbee-io/qbee-agent/app/utils"
)

type DockerImages struct {
	Images []DockerImage `json:"items"`
}

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
func CollectDockerImagesInventory() (*DockerImages, error) {
	if !HasDocker() {
		return nil, nil
	}

	cmd := []string{"docker", "image", "ls", "--no-trunc", "--all", "--format", dockerImagesFormat}

	images := make([]DockerImage, 0)

	err := utils.ForLinesInCommandOutput(cmd, func(line string) error {
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
