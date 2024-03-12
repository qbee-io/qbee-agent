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

package agent

import (
	"context"
	"fmt"

	"go.qbee.io/agent/app"
	"go.qbee.io/agent/app/inventory"
)

// doInventories collects all inventories and delivers them to the device hub API.
func (agent *Agent) doInventories(ctx context.Context) error {
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
		"rauc":              agent.doRaucInventory,
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
	systemInventory, err := inventory.CollectSystemInventory(agent.IsTPMEnabled())
	if err != nil {
		return err
	}

	systemInventory.System.LastConfigCommitID = agent.Configuration.CurrentCommitID()
	systemInventory.System.LastConfigUpdate = fmt.Sprintf("%d", agent.Configuration.ConfigChangeTimestamp())
	systemInventory.System.LastPolicyUpdate = systemInventory.System.LastConfigUpdate
	systemInventory.System.AgentVersion = app.Version

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

// doRaucInventory collects RAUC inventory - if enabled - and delivers it to the device hub API.
func (agent *Agent) doRaucInventory(ctx context.Context) error {
	raucInventory, err := inventory.CollectRaucInventory(ctx)
	if err != nil {
		return err
	}

	// Do not send anything if rauc is not installed
	if raucInventory == nil {
		return nil
	}

	return agent.Inventory.Send(ctx, inventory.TypeRauc, raucInventory)
}
