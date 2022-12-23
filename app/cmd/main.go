package cmd

import "github.com/qbee-io/qbee-agent/app/agent"

const (
	mainConfigDirOption = "config-dir"
)

var Main = Command{
	Description: "Qbee Agent Command-Line Tool",
	Options: []Option{
		{
			Name:    mainConfigDirOption,
			Short:   "c",
			Help:    "Configuration directory.",
			Default: agent.DefaultConfigDir,
		},
	},
	SubCommands: map[string]Command{
		"bootstrap": bootstrapCommand,
		"start":     startCommand,
		"inventory": inventoryCommand,
	},
}
