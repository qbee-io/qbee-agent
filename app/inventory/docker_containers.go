package inventory

import (
	"encoding/json"
	"fmt"

	"github.com/qbee-io/qbee-agent/app/utils"
)

type DockerContainers struct {
	Containers []DockerContainer `json:"items"`
}

type DockerContainer struct {
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

const dockerContainersFormat = `{"id":"{{.ID}}","names":"{{.Names}}","image":"{{.Image}}","command":{{.Command}},` +
	`"created_at":"{{.CreatedAt}}","status":"{{.Status}}","ports":"{{.Ports}}","size":"{{.Size}}",` +
	`"mounts":"{{.Mounts}}", "networks":"{{.Networks}}"}`

// CollectDockerContainersInventory returns populated DockerContainers inventory based on current system status.
func CollectDockerContainersInventory() (*DockerContainers, error) {
	if !HasDocker() {
		return nil, nil
	}

	cmd := []string{"docker", "container", "ls", "--no-trunc", "--all", "--format", dockerContainersFormat}

	containers := make([]DockerContainer, 0)

	err := utils.ForLinesInCommandOutput(cmd, func(line string) error {
		container := new(DockerContainer)

		if err := json.Unmarshal([]byte(line), container); err != nil {
			return fmt.Errorf("error decoding docker container: %w", err)
		}

		containers = append(containers, *container)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error listing docker containers: %w", err)
	}

	dockerContainers := &DockerContainers{
		Containers: containers,
	}

	return dockerContainers, nil
}
