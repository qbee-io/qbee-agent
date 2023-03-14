package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/qbee-io/qbee-agent/app"
	"github.com/qbee-io/qbee-agent/app/inventory"
	"github.com/qbee-io/qbee-agent/app/log"
)

// doInventories collects all inventories and delivers them to the device hub API.
func (agent *Agent) doInventories(ctx context.Context, waitGroup *sync.WaitGroup) {
	agent.doSystemInventory(ctx, waitGroup)
	agent.doUsersInventory(ctx, waitGroup)
	agent.doPortsInventory(ctx, waitGroup)
	agent.doDockerContainersInventory(ctx, waitGroup)
	agent.doDockerImagesInventory(ctx, waitGroup)
	agent.doDockerVolumesInventory(ctx, waitGroup)
	agent.doDockerNetworksInventory(ctx, waitGroup)
	agent.doSoftwareInventory(ctx, waitGroup)
	agent.doProcessInventory(ctx, waitGroup)
}

// doSystemInventory collects system inventory and delivers it to the device hub API.
func (agent *Agent) doSystemInventory(ctx context.Context, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		systemInventory, err := inventory.CollectSystemInventory()
		if err != nil {
			log.Errorf("failed to collect system inventory: %v", err)
			return
		}

		systemInventory.System.LastConfigCommitID = agent.Configuration.CurrentCommitID()
		systemInventory.System.LastConfigUpdate = fmt.Sprintf("%d", agent.Configuration.ConfigChangeTimestamp())
		systemInventory.System.LastPolicyUpdate = systemInventory.System.LastConfigUpdate
		systemInventory.System.AgentVersion = app.Version

		if err = agent.Inventory.Send(ctx, inventory.TypeSystem, systemInventory); err != nil {
			log.Errorf("failed to send system inventory: %v", err)
		}
	}()
}

// doUsersInventory collects users inventory and delivers it to the device hub API.
func (agent *Agent) doUsersInventory(ctx context.Context, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		usersInventory, err := inventory.CollectUsersInventory()
		if err != nil {
			log.Errorf("failed to collect users inventory: %v", err)
			return
		}

		if err = agent.Inventory.Send(ctx, inventory.TypeUsers, usersInventory); err != nil {
			log.Errorf("failed to send users inventory: %v", err)
		}
	}()
}

// doPortsInventory collects ports inventory and delivers it to the device hub API.
func (agent *Agent) doPortsInventory(ctx context.Context, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		portsInventory, err := inventory.CollectPortsInventory()
		if err != nil {
			log.Errorf("failed to collect ports inventory: %v", err)
			return
		}

		if err = agent.Inventory.Send(ctx, inventory.TypePorts, portsInventory); err != nil {
			log.Errorf("failed to send ports inventory: %v", err)
		}
	}()
}

// doDockerContainersInventory collects docker containers inventory and delivers it to the device hub API.
func (agent *Agent) doDockerContainersInventory(ctx context.Context, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		dockerContainersInventory, err := inventory.CollectDockerContainersInventory(ctx)
		if err != nil {
			log.Errorf("failed to collect docker containers inventory: %v", err)
			return
		}

		if err = agent.Inventory.Send(ctx, inventory.TypeDockerContainers, dockerContainersInventory); err != nil {
			log.Errorf("failed to send docker containers inventory: %v", err)
		}
	}()
}

// doDockerImagesInventory collects docker images inventory and delivers it to the device hub API.
func (agent *Agent) doDockerImagesInventory(ctx context.Context, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		dockerImagesInventory, err := inventory.CollectDockerImagesInventory(ctx)
		if err != nil {
			log.Errorf("failed to collect docker images inventory: %v", err)
			return
		}

		if err = agent.Inventory.Send(ctx, inventory.TypeDockerImages, dockerImagesInventory); err != nil {
			log.Errorf("failed to send docker images inventory: %v", err)
		}
	}()
}

// doDockerVolumesInventory collects docker volumes inventory and delivers it to the device hub API.
func (agent *Agent) doDockerVolumesInventory(ctx context.Context, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		dockerVolumesInventory, err := inventory.CollectDockerVolumesInventory(ctx)
		if err != nil {
			log.Errorf("failed to collect docker volumes inventory: %v", err)
			return
		}

		if err = agent.Inventory.Send(ctx, inventory.TypeDockerVolumes, dockerVolumesInventory); err != nil {
			log.Errorf("failed to send docker volumes inventory: %v", err)
		}
	}()
}

// doDockerNetworksInventory collects docker networks inventory and delivers it to the device hub API.
func (agent *Agent) doDockerNetworksInventory(ctx context.Context, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()

		dockerNetworksInventory, err := inventory.CollectDockerNetworksInventory(ctx)
		if err != nil {
			log.Errorf("failed to collect docker networks inventory: %v", err)
			return
		}

		if err = agent.Inventory.Send(ctx, inventory.TypeDockerNetworks, dockerNetworksInventory); err != nil {
			log.Errorf("failed to send docker networks inventory: %v", err)
		}
	}()
}

// doSoftwareInventory collects software inventory - if enabled - and delivers it to the device hub API.
func (agent *Agent) doSoftwareInventory(ctx context.Context, waitGroup *sync.WaitGroup) {
	if !agent.Configuration.CollectSoftwareInventory() {
		return
	}

	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()

		softwareInventory, err := inventory.CollectSoftwareInventory(ctx)
		if err != nil {
			log.Errorf("failed to collect software inventory: %v", err)
			return
		}

		if err = agent.Inventory.Send(ctx, inventory.TypeSoftware, softwareInventory); err != nil {
			log.Errorf("failed to send software inventory: %v", err)
		}
	}()
}

// doProcessInventory collects process inventory - if enabled - and delivers it to the device hub API.
func (agent *Agent) doProcessInventory(ctx context.Context, waitGroup *sync.WaitGroup) {
	if !agent.Configuration.CollectProcessInventory() {
		return
	}

	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()

		processesInventory, err := inventory.CollectProcessesInventory()
		if err != nil {
			log.Errorf("failed to collect process inventory: %v", err)
			return
		}

		if err = agent.Inventory.Send(ctx, inventory.TypeProcesses, processesInventory); err != nil {
			log.Errorf("failed to send process inventory: %v", err)
		}
	}()
}
