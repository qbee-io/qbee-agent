package inventory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/inventory/software"
)

// Service provides methods for collecting and delivering inventory data.
type Service struct {
	api                       *api.Client
	deliveredInventoryDigests map[Type]string
}

// New returns a new instance of inventory Service.
func New(apiClient *api.Client) *Service {
	return &Service{
		api:                       apiClient,
		deliveredInventoryDigests: make(map[Type]string),
	}
}

// SendSystemInventory gathers system inventory and sends them to the device hub API.
func (srv *Service) SendSystemInventory(ctx context.Context, currentConfigCommitID string) error {
	systemInventory, err := CollectSystemInventory()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}

	systemInventory.System.LastConfigCommitID = currentConfigCommitID

	return srv.Send(ctx, TypeSystem, systemInventory)
}

// sendPortsInventory gathers open ports inventory and sends them to the device hub API.
func (srv *Service) sendPortsInventory(ctx context.Context) error {
	portsInventory, err := CollectPortsInventory()
	if err != nil {
		return fmt.Errorf("error collecting open ports: %w", err)
	}

	return srv.Send(ctx, TypePorts, portsInventory)
}

// sendProcessesInventory gathers running processes inventory and sends them to the device hub API.
func (srv *Service) sendProcessesInventory(ctx context.Context) error {
	processesInventory, err := CollectProcessesInventory()
	if err != nil {
		return fmt.Errorf("error collecting running processes: %w", err)
	}

	return srv.Send(ctx, TypeProcesses, processesInventory)
}

// sendUsersInventory gathers running processes inventory and sends them to the device hub API.
func (srv *Service) sendUsersInventory(ctx context.Context) error {
	usersInventory, err := CollectUsersInventory()
	if err != nil {
		return fmt.Errorf("error collecting running users: %w", err)
	}

	return srv.Send(ctx, TypeUsers, usersInventory)
}

// sendSoftwareInventory gathers software inventory and sends them to the device hub API.
func (srv *Service) sendSoftwareInventory(ctx context.Context) error {
	for pkgManager := range software.PackageManagers {
		softwareInventory, err := CollectSoftwareInventory(pkgManager)
		if err != nil {
			return fmt.Errorf("error collecting software inventory: %w", err)
		}

		// skip unsupported package managers
		if softwareInventory == nil {
			continue
		}

		if err = srv.Send(ctx, TypeSoftware, softwareInventory); err != nil {
			return fmt.Errorf("error sending software inventory: %w", err)
		}
	}

	return nil
}

// sendDockerContainersInventory gathers docker containers inventory and sends them to the device hub API.
func (srv *Service) sendDockerContainersInventory(ctx context.Context) error {
	if !HasDocker() {
		return nil
	}

	dockerContainersInventory, err := CollectDockerContainersInventory()
	if err != nil {
		return fmt.Errorf("error collecting docker containers inventory: %w", err)
	}

	if err = srv.Send(ctx, TypeDockerContainers, dockerContainersInventory); err != nil {
		return fmt.Errorf("error sending docker containers inventory: %w", err)
	}

	return nil
}

// sendDockerImagesInventory gathers docker images inventory and sends them to the device hub API.
func (srv *Service) sendDockerImagesInventory(ctx context.Context) error {
	if !HasDocker() {
		return nil
	}

	dockerImagesInventory, err := CollectDockerImagesInventory()
	if err != nil {
		return fmt.Errorf("error collecting docker images inventory: %w", err)
	}

	if err = srv.Send(ctx, TypeDockerImages, dockerImagesInventory); err != nil {
		return fmt.Errorf("error sending docker images: %w", err)
	}

	return nil
}

// sendDockerNetworksInventory gathers docker networks inventory and sends them to the device hub API.
func (srv *Service) sendDockerNetworksInventory(ctx context.Context) error {
	if !HasDocker() {
		return nil
	}

	dockerNetworksInventory, err := CollectDockerNetworksInventory()
	if err != nil {
		return fmt.Errorf("error collecting docker networks inventory: %w", err)
	}

	if err = srv.Send(ctx, TypeDockerNetworks, dockerNetworksInventory); err != nil {
		return fmt.Errorf("error sending docker networks: %w", err)
	}

	return nil
}

// sendDockerVolumesInventory gathers docker volumes inventory and sends them to the device hub API.
func (srv *Service) sendDockerVolumesInventory(ctx context.Context) error {
	if !HasDocker() {
		return nil
	}

	dockerVolumesInventory, err := CollectDockerVolumesInventory()
	if err != nil {
		return fmt.Errorf("error collecting docker volumes inventory: %w", err)
	}

	if err = srv.Send(ctx, TypeDockerVolumes, dockerVolumesInventory); err != nil {
		return fmt.Errorf("error sending docker volumes: %w", err)
	}

	return nil
}

// Send delivers inventory to device hub if it has changes since last delivery.
func (srv *Service) Send(ctx context.Context, inventoryType Type, inventoryData any) error {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(inventoryData); err != nil {
		return fmt.Errorf("error marshaling %s inventory data: %w", inventoryType, err)
	}

	currentDigest := fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))

	// if previously delivered inventory matches current one, don't report it
	if previousDigest, ok := srv.deliveredInventoryDigests[inventoryType]; ok && previousDigest == currentDigest {
		return nil
	}

	if err := srv.send(ctx, inventoryType, buf); err != nil {
		return fmt.Errorf("error sending %s inventory request: %w", inventoryType, err)
	}

	srv.deliveredInventoryDigests[inventoryType] = currentDigest

	return nil
}
