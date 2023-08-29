package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"qbee.io/platform/utils/flags"

	"github.com/qbee-io/qbee-agent/app/agent"
	"github.com/qbee-io/qbee-agent/app/inventory"
)

const (
	inventoryTypeOption   = "type"
	inventoryDryRunOption = "dry-run"
)

var inventoryCommand = flags.Command{
	Description: "Send inventory data to the Device Hub API.",
	Options: []flags.Option{
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
	Target: func(opts flags.Options) error {
		inventoryType := inventory.Type(opts[inventoryTypeOption])
		dryRun := opts[inventoryDryRunOption] == "true"

		ctx := context.Background()

		var err error
		var inventoryData any

		switch inventoryType {
		case inventory.TypeSystem:
			inventoryData, err = inventory.CollectSystemInventory()
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
		default:
			return fmt.Errorf("unsupported inventory type")
		}

		if err != nil {
			return err
		}

		if dryRun {
			return json.NewEncoder(os.Stdout).Encode(inventoryData)
		}

		var cfg *agent.Config
		if cfg, err = loadConfig(opts); err != nil {
			return err
		}

		var deviceAgent *agent.Agent
		if deviceAgent, err = agent.New(cfg); err != nil {
			return fmt.Errorf("error initializing the agent: %w", err)
		}

		return deviceAgent.Inventory.Send(ctx, inventoryType, inventoryData)
	},
}
