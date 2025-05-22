package cmd

import (
	"fmt"

	"go.qbee.io/agent/app/agent"
	"go.qbee.io/agent/app/utils/cmd"
)

/*
# install the service

$ sc create qbee-agent binPath= "C:\Users\jonhenrik\qbee-agent service --start" start= auto
*/

var serviceCommand = cmd.Command{
	Description: "Manage the agent service (windows only).",
	Options: []cmd.Option{
		{
			Name:  "install",
			Short: "i",
			Help:  "Install the agent service.",
			Flag:  "true",
		},
		{
			Name:  "uninstall",
			Short: "u",
			Help:  "Uninstall the agent service.",
			Flag:  "true",
		},
		{
			Name:  "start",
			Short: "s",
			Help:  "Start the agent service.",
			Flag:  "true",
		},
		{
			Name:  "stop",
			Short: "t",
			Help:  "Stop the agent service.",
			Flag:  "true",
		},
		{
			Name:  "restart",
			Short: "r",
			Help:  "Restart the agent service.",
			Flag:  "true",
		},
	},
	Target: func(opts cmd.Options) error {
		// Start the service
		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		start := opts["start"] == "true"
		//stop := opts["stop"] == "true"

		if start {
			return agent.RunService(cfg)
		}
		return fmt.Errorf("invalid service command")
	},
}
