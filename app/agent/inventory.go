package agent

import (
	"context"
	"fmt"

	"github.com/qbee-io/qbee-agent/app"
	"github.com/qbee-io/qbee-agent/app/inventory"
)

// doInventories collects all inventories and delivers them to the device hub API.
func (agent *Agent) doInventories(ctx context.Context) error {
	agent.inventoryLock.Lock()
	defer agent.inventoryLock.Unlock()

	inventories := map[string]func(ctx context.Context) error{
		"system":            agent.doSystemInventory,
		"users":             agent.doUsersInventory,
		"ports":             agent.doPortsInventory,
		"docker-containers": agent.doDockerContainersInventory,
		"docker-images":     agent.doDockerImagesInventory,
		"docker-volumes":    agent.doDockerVolumesInventory,
		"docker-networks":   agent.doDockerNetworksInventory,
		"software":          agent.doSoftwareInventory,
		"process":           agent.doProcessInventory,
	}

	for name, fn := range inventories {
		if err := fn(ctx); err != nil {
			return fmt.Errorf("failed to do %s inventory: %w", name, err)
		}
	}

	return nil
}

// doInventoriesSimple collects all inventories and delivers them to the device hub API.
func (agent *Agent) doInventoriesSimple(ctx context.Context) error {
	agent.inventoryLock.Lock()
	defer agent.inventoryLock.Unlock()

	inventories := map[string]func(ctx context.Context) error{
		"system":  agent.doSystemInventory,
		"users":   agent.doUsersInventory,
		"ports":   agent.doPortsInventory,
		"process": agent.doProcessInventory,
	}

	for name, fn := range inventories {
		if err := fn(ctx); err != nil {
			return fmt.Errorf("failed to do %s inventory: %w", name, err)
		}
	}

	return nil
}

// doSystemInventory collects system inventory and delivers it to the device hub API.
func (agent *Agent) doSystemInventory(ctx context.Context) error {
	systemInventory, err := inventory.CollectSystemInventory()
	if err != nil {
		return err
	}

	systemInventory.System.LastConfigCommitID = agent.Configuration.CurrentCommitID()
	systemInventory.System.LastConfigUpdate = fmt.Sprintf("%d", agent.Configuration.ConfigChangeTimestamp())
	systemInventory.System.LastPolicyUpdate = systemInventory.System.LastConfigUpdate
	systemInventory.System.AgentVersion = app.Version
	systemInventory.System.AutoUpdateEnabled = agent.cfg.AutoUpdate

	return agent.Inventory.Send(ctx, inventory.TypeSystem, systemInventory)
}

// doUsersInventory collects users inventory and delivers it to the device hub API.
func (agent *Agent) doUsersInventory(ctx context.Context) error {
	usersInventory, err := inventory.CollectUsersInventory()
	if err != nil {
		return err
	}

	return agent.Inventory.Send(ctx, inventory.TypeUsers, usersInventory)
}

// doPortsInventory collects ports inventory and delivers it to the device hub API.
func (agent *Agent) doPortsInventory(ctx context.Context) error {
	portsInventory, err := inventory.CollectPortsInventory()
	if err != nil {
		return err
	}

	return agent.Inventory.Send(ctx, inventory.TypePorts, portsInventory)
}

// doDockerContainersInventory collects docker containers inventory and delivers it to the device hub API.
func (agent *Agent) doDockerContainersInventory(ctx context.Context) error {
	dockerContainersInventory, err := inventory.CollectDockerContainersInventory(ctx)
	if err != nil {
		return err
	}

	return agent.Inventory.Send(ctx, inventory.TypeDockerContainers, dockerContainersInventory)
}

// doDockerImagesInventory collects docker images inventory and delivers it to the device hub API.
func (agent *Agent) doDockerImagesInventory(ctx context.Context) error {
	dockerImagesInventory, err := inventory.CollectDockerImagesInventory(ctx)
	if err != nil {
		return err

	}

	return agent.Inventory.Send(ctx, inventory.TypeDockerImages, dockerImagesInventory)
}

// doDockerVolumesInventory collects docker volumes inventory and delivers it to the device hub API.
func (agent *Agent) doDockerVolumesInventory(ctx context.Context) error {
	dockerVolumesInventory, err := inventory.CollectDockerVolumesInventory(ctx)
	if err != nil {
		return err
	}

	return agent.Inventory.Send(ctx, inventory.TypeDockerVolumes, dockerVolumesInventory)
}

// doDockerNetworksInventory collects docker networks inventory and delivers it to the device hub API.
func (agent *Agent) doDockerNetworksInventory(ctx context.Context) error {
	dockerNetworksInventory, err := inventory.CollectDockerNetworksInventory(ctx)
	if err != nil {
		return err
	}

	return agent.Inventory.Send(ctx, inventory.TypeDockerNetworks, dockerNetworksInventory)
}

// doSoftwareInventory collects software inventory - if enabled - and delivers it to the device hub API.
func (agent *Agent) doSoftwareInventory(ctx context.Context) error {
	if !agent.Configuration.CollectSoftwareInventory() {
		return nil
	}

	softwareInventory, err := inventory.CollectSoftwareInventory(ctx)
	if err != nil {
		return err
	}

	return agent.Inventory.Send(ctx, inventory.TypeSoftware, softwareInventory)
}

// doProcessInventory collects process inventory - if enabled - and delivers it to the device hub API.
func (agent *Agent) doProcessInventory(ctx context.Context) error {
	if !agent.Configuration.CollectProcessInventory() {
		return nil
	}

	processesInventory, err := inventory.CollectProcessesInventory()
	if err != nil {
		return err
	}

	return agent.Inventory.Send(ctx, inventory.TypeProcesses, processesInventory)
}
