package cmd

import "github.com/qbee-io/qbee-agent/app/agent"

const (
	mainConfigDirOption = "config-dir"
	mainStateDirOption  = "state-dir"
)

const (
	DefaultConfigDir = "/etc/qbee"
	DefaultStateDir  = "/var/lib/qbee"
)

var Main = Command{
	Description: "Qbee Agent Command-Line Tool",
	Options: []Option{
		{
			Name:    mainConfigDirOption,
			Short:   "c",
			Help:    "Configuration directory.",
			Default: DefaultConfigDir,
		},
		{
			Name:    mainStateDirOption,
			Short:   "s",
			Help:    "State directory.",
			Default: DefaultStateDir,
		},
	},
	SubCommands: map[string]Command{
		"bootstrap": bootstrapCommand,
		"start":     startCommand,
		"inventory": inventoryCommand,
		"config":    configCommand,
	},
}

// loadConfig is a helper method to load agent's config based on provided command-line options.
func loadConfig(opts Options) (*agent.Config, error) {
	return agent.LoadConfig(opts[mainConfigDirOption], opts[mainStateDirOption])
}
