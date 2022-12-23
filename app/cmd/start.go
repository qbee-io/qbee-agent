package cmd

import (
	"context"

	"github.com/qbee-io/qbee-agent/app/agent"
)

const (
	startDisableAutoUpdateOption = "no-auto-update"
)

var startCommand = Command{
	Description: "Start the agent process.",

	Options: []Option{
		{
			Name:  startDisableAutoUpdateOption,
			Short: "n",
			Help:  "Disable auto-update.",
			Flag:  "true",
		},
	},

	Target: func(opts Options) error {
		disableAutoUpdate := opts[startDisableAutoUpdateOption] == "true"

		ctx := context.Background()

		cfg, err := agent.LoadConfig(opts[mainConfigDirOption])
		if err != nil {
			return err
		}

		if disableAutoUpdate || !cfg.AutoUpdate {
			return agent.Start(ctx, cfg)
		}

		return agent.StartWithAutoUpdate(ctx, cfg)
	},
}
