package cmd

import (
	"github.com/qbee-io/qbee-agent/app/agent"
	"github.com/qbee-io/qbee-agent/app/log"
)

const (
	mainConfigDirOption = "config-dir"
	mainStateDirOption  = "state-dir"
	mainLogLevel        = "log-level"
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
		{
			Name:    mainLogLevel,
			Short:   "l",
			Help:    "Logging level: DEBUG, INFO or ERROR.",
			Default: "INFO",
		},
	},
	SubCommands: map[string]Command{
		"bootstrap": bootstrapCommand,
		"start":     startCommand,
		"inventory": inventoryCommand,
		"config":    configCommand,
		"version":   versionCommand,
	},
}

// loadConfig is a helper method to load agent's config based on provided command-line options.
func loadConfig(opts Options) (*agent.Config, error) {
	switch opts[mainLogLevel] {
	case "DEBUG":
		log.SetLevel(log.DEBUG)
	case "INFO":
		log.SetLevel(log.INFO)
	case "ERROR":
		log.SetLevel(log.ERROR)
	}

	return agent.LoadConfig(opts[mainConfigDirOption], opts[mainStateDirOption])
}
