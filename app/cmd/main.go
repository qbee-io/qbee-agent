package cmd

import (
	"github.com/qbee-io/qbee-agent/app/agent"
	"github.com/qbee-io/qbee-agent/app/log"
)

const (
	mainConfigDirOption = "config-dir"
	mainCacheDirOption  = "cache-dir"
	mainLogLevel        = "log-level"
)

const (
	DefaultConfigDir = "/etc/qbee"
	DefaultCacheDir  = "/var/cache/qbee"
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
			Name:    mainCacheDirOption,
			Short:   "x",
			Help:    "Cache directory.",
			Default: DefaultCacheDir,
		},
		{
			Name:    mainLogLevel,
			Short:   "l",
			Help:    "Logging level: DEBUG, INFO, WARNING or ERROR.",
			Default: "INFO",
		},
	},
	SubCommands: map[string]Command{
		"bootstrap": bootstrapCommand,
		"config":    configCommand,
		"inventory": inventoryCommand,
		"start":     startCommand,
		"update":    updateCommand,
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
	case "WARNING":
		log.SetLevel(log.WARNING)
	case "ERROR":
		log.SetLevel(log.ERROR)
	}

	return agent.LoadConfig(opts[mainConfigDirOption], opts[mainCacheDirOption])
}
