package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/qbee-io/qbee-agent/app/agent"
	"github.com/qbee-io/qbee-agent/app/inventory"
	"github.com/qbee-io/qbee-agent/app/inventory/software"
)

const (
	inventoryTypeOption       = "type"
	inventoryPkgManagerOption = "pkg-manager"
	inventoryDryRunOption     = "dry-run"
)

var inventoryCommand = Command{
	Description: "Send inventory data to the Device Hub API.",
	Options: []Option{
		{
			Name:     inventoryTypeOption,
			Short:    "t",
			Help:     "Inventory type.",
			Required: true,
			Default:  "system",
		},
		{
			Name:    inventoryPkgManagerOption,
			Short:   "p",
			Help:    "Package Manager (for software inventory).",
			Default: string(software.DebPackageManagerType),
		},
		{
			Name:  inventoryDryRunOption,
			Short: "d",
			Help:  "Don't send inventory. Just dump as JSON to standard output.",
			Flag:  "true",
		},
	},
	Target: func(opts Options) error {
		inventoryType := inventory.Type(opts[inventoryTypeOption])
		pkgManager := software.PackageManagerType(opts[inventoryPkgManagerOption])
		dryRun := opts[inventoryDryRunOption] == "true"

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
			inventoryData, err = inventory.CollectSoftwareInventory(pkgManager)
		case inventory.TypeDockerContainers:
			inventoryData, err = inventory.CollectDockerContainersInventory()
		case inventory.TypeDockerImages:
			inventoryData, err = inventory.CollectDockerImagesInventory()
		case inventory.TypeDockerNetworks:
			inventoryData, err = inventory.CollectDockerNetworksInventory()
		case inventory.TypeDockerVolumes:
			inventoryData, err = inventory.CollectDockerVolumesInventory()
		default:
			return fmt.Errorf("unsupported inventory type")
		}

		if err != nil {
			return err
		}

		if dryRun {
			return json.NewEncoder(os.Stdout).Encode(inventoryData)
		}

		ctx := context.Background()
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
