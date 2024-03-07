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

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.qbee.io/agent/app/agent"
	"go.qbee.io/agent/app/inventory"
	"go.qbee.io/agent/app/utils/cmd"
)

const (
	inventoryTypeOption   = "type"
	inventoryDryRunOption = "dry-run"
)

var inventoryCommand = cmd.Command{
	Description: "Send inventory data to the Device Hub API.",
	Options: []cmd.Option{
		{
			Name:     inventoryTypeOption,
			Short:    "t",
			Help:     "Inventory type.",
			Required: true,
			Default:  "system",
		},
		{
			Name:  inventoryDryRunOption,
			Short: "d",
			Help:  "Don't send inventory. Just dump as JSON to standard output.",
			Flag:  "true",
		},
	},
	Target: func(opts cmd.Options) error {
		inventoryType := inventory.Type(opts[inventoryTypeOption])
		dryRun := opts[inventoryDryRunOption] == "true"

		ctx := context.Background()

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		var deviceAgent *agent.Agent
		if deviceAgent, err = agent.New(cfg); err != nil {
			return fmt.Errorf("error initializing the agent: %w", err)
		}

		var inventoryData any

		switch inventoryType {
		case inventory.TypeSystem:
			inventoryData, err = inventory.CollectSystemInventory(deviceAgent.IsTPMEnabled())
		case inventory.TypePorts:
			inventoryData, err = inventory.CollectPortsInventory()
		case inventory.TypeProcesses:
			inventoryData, err = inventory.CollectProcessesInventory()
		case inventory.TypeUsers:
			inventoryData, err = inventory.CollectUsersInventory()
		case inventory.TypeSoftware:
			inventoryData, err = inventory.CollectSoftwareInventory(ctx)
		case inventory.TypeDockerContainers:
			inventoryData, err = inventory.CollectDockerContainersInventory(ctx)
		case inventory.TypeDockerImages:
			inventoryData, err = inventory.CollectDockerImagesInventory(ctx)
		case inventory.TypeDockerNetworks:
			inventoryData, err = inventory.CollectDockerNetworksInventory(ctx)
		case inventory.TypeDockerVolumes:
			inventoryData, err = inventory.CollectDockerVolumesInventory(ctx)
		case inventory.TypeRauc:
			inventoryData, err = inventory.CollectRaucInventory(ctx)
		default:
			return fmt.Errorf("unsupported inventory type")
		}

		if err != nil {
			return err
		}

		if dryRun {
			return json.NewEncoder(os.Stdout).Encode(inventoryData)
		}

		return deviceAgent.Inventory.Send(ctx, inventoryType, inventoryData)
	},
}
