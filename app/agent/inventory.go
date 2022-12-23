package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/qbee-io/qbee-agent/app/inventory"
	"github.com/qbee-io/qbee-agent/app/inventory/software"
)

type InventoryType string

const (
	SystemInventoryType           InventoryType = "system"
	PortsInventoryType            InventoryType = "ports"
	ProcessesInventoryType        InventoryType = "processes"
	UsersInventoryType            InventoryType = "users"
	SoftwareInventoryType         InventoryType = "software"
	DockerContainersInventoryType InventoryType = "docker_containers"
	DockerImagesInventoryType     InventoryType = "docker_images"
	DockerNetworksInventoryType   InventoryType = "docker_networks"
	DockerVolumesInventoryType    InventoryType = "docker_volumes"
)

// sendSystemInventory gathers system inventory and sends them to the device hub API.
func (agent *Agent) sendSystemInventory(ctx context.Context) error {
	systemInventory, err := inventory.CollectSystemInventory()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}

	systemInventory.System.AgentVersion = Version
	systemInventory.System.LastConfigCommitID = agent.lastConfigCommitID

	return agent.sendInventory(ctx, SystemInventoryType, systemInventory)
}

// sendPortsInventory gathers open ports inventory and sends them to the device hub API.
func (agent *Agent) sendPortsInventory(ctx context.Context) error {
	portsInventory, err := inventory.CollectPortsInventory()
	if err != nil {
		return fmt.Errorf("error collecting open ports: %w", err)
	}

	return agent.sendInventory(ctx, PortsInventoryType, portsInventory)
}

// sendProcessesInventory gathers running processes inventory and sends them to the device hub API.
func (agent *Agent) sendProcessesInventory(ctx context.Context) error {
	processesInventory, err := inventory.CollectProcessesInventory()
	if err != nil {
		return fmt.Errorf("error collecting running processes: %w", err)
	}

	return agent.sendInventory(ctx, ProcessesInventoryType, processesInventory)
}

// sendUsersInventory gathers running processes inventory and sends them to the device hub API.
func (agent *Agent) sendUsersInventory(ctx context.Context) error {
	usersInventory, err := inventory.CollectUsersInventory()
	if err != nil {
		return fmt.Errorf("error collecting running users: %w", err)
	}

	return agent.sendInventory(ctx, UsersInventoryType, usersInventory)
}

// sendSoftwareInventory gathers software inventory and sends them to the device hub API.
func (agent *Agent) sendSoftwareInventory(ctx context.Context) error {
	for pkgManager := range software.PackageManagers {
		softwareInventory, err := inventory.CollectSoftwareInventory(pkgManager)
		if err != nil {
			return fmt.Errorf("error collecting software inventory: %w", err)
		}

		// skip unsupported package managers
		if softwareInventory == nil {
			continue
		}

		if err = agent.sendInventory(ctx, SoftwareInventoryType, softwareInventory); err != nil {
			return fmt.Errorf("error sending software inventory: %w", err)
		}
	}

	return nil
}

// sendDockerContainersInventory gathers docker containers inventory and sends them to the device hub API.
func (agent *Agent) sendDockerContainersInventory(ctx context.Context) error {
	if !inventory.HasDocker() {
		return nil
	}

	dockerContainersInventory, err := inventory.CollectDockerContainersInventory()
	if err != nil {
		return fmt.Errorf("error collecting docker containers inventory: %w", err)
	}

	if err = agent.sendInventory(ctx, DockerContainersInventoryType, dockerContainersInventory); err != nil {
		return fmt.Errorf("error sending docker containers inventory: %w", err)
	}

	return nil
}

// sendDockerImagesInventory gathers docker containers inventory and sends them to the device hub API.
func (agent *Agent) sendDockerImagesInventory(ctx context.Context) error {
	if !inventory.HasDocker() {
		return nil
	}

	dockerImagesInventory, err := inventory.CollectDockerImagesInventory()
	if err != nil {
		return fmt.Errorf("error collecting docker images inventory: %w", err)
	}

	if err = agent.sendInventory(ctx, DockerImagesInventoryType, dockerImagesInventory); err != nil {
		return fmt.Errorf("error sending docker images: %w", err)
	}

	return nil
}

// sendInventory delivers inventory to device hub if it has changes since last delivery.
func (agent *Agent) sendInventory(ctx context.Context, inventoryType InventoryType, inventoryData any) error {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(inventoryData); err != nil {
		return fmt.Errorf("error marshaling %s inventory data: %w", inventoryType, err)
	}

	currentDigest := fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))

	// if previously delivered inventory matches current one, don't report it
	if previousDigest, ok := agent.deliveredInventoryDigests[inventoryType]; ok && previousDigest == currentDigest {
		return nil
	}

	endpoint := fmt.Sprintf("/v1/org/device/auth/inventory/%s", inventoryType)

	response, err := agent.apiRequest(ctx, http.MethodPut, endpoint, buf)
	if err != nil {
		return fmt.Errorf("error sending %s inventory request: %w", inventoryType, err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(response.Body)
		return fmt.Errorf("unexpected response status: %d %s", response.StatusCode, responseBody)
	}

	agent.deliveredInventoryDigests[inventoryType] = currentDigest

	return nil
}
