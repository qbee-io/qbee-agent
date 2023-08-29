package cmd

import (
	"qbee.io/platform/utils/flags"

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

var Main = flags.Command{
	Description: "Qbee Agent Command-Line Tool",
	Options: []flags.Option{
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
			Help:    "Logging level: DEBUG, INFO, WARNING or ERROR.",
			Default: "INFO",
		},
	},
	SubCommands: map[string]flags.Command{
		"bootstrap": bootstrapCommand,
		"config":    configCommand,
		"inventory": inventoryCommand,
		"start":     startCommand,
		"update":    updateCommand,
		"version":   versionCommand,
	},
}

// loadConfig is a helper method to load agent's config based on provided command-line options.
func loadConfig(opts flags.Options) (*agent.Config, error) {
	switch opts[mainLogLevel] {
	case "DEBUG":
		log.SetLevel(log.DEBUG)
	case "INFO":
		log.SetLevel(log.INFO)
	case "WARNING":
		log.SetLevel(log.WARNING)
	case "ERROR":
		log.SetLevel(log.ERROR)
	}

	return agent.LoadConfig(opts[mainConfigDirOption], opts[mainStateDirOption])
}
