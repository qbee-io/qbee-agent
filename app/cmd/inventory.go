package cmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/qbee-io/qbee-agent/app/agent"
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
		inventoryType := agent.InventoryType(opts[inventoryTypeOption])
		pkgManager := software.PackageManagerType(opts[inventoryPkgManagerOption])
		dryRun := opts[inventoryDryRunOption] == "true"

		inventoryData, err := agent.CollectInventory(inventoryType, pkgManager)
		if err != nil {
			return err
		}

		if dryRun {
			return json.NewEncoder(os.Stdout).Encode(inventoryData)
		}

		ctx := context.Background()
		var cfg *agent.Config
		if cfg, err = agent.LoadConfig(opts[mainConfigDirOption]); err != nil {
			return err
		}

		return agent.SendInventory(ctx, cfg, inventoryType, inventoryData)
	},
}
