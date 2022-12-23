package inventory

import (
	"encoding/json"
	"fmt"

	"github.com/qbee-io/qbee-agent/app/utils"
)

type DockerVolumes struct {
	Volumes []DockerVolume `json:"items"`
}

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
func CollectDockerVolumesInventory() (*DockerVolumes, error) {
	if !HasDocker() {
		return nil, nil
	}

	cmd := []string{"docker", "volume", "ls", "--format", dockerVolumesFormat}

	volumes := make([]DockerVolume, 0)

	err := utils.ForLinesInCommandOutput(cmd, func(line string) error {
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
