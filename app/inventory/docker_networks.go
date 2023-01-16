package inventory

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qbee-io/qbee-agent/app/utils"
)

const TypeDockerNetworks Type = "docker_networks"

type DockerNetworks struct {
	Networks []DockerNetwork `json:"items"`
}

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
