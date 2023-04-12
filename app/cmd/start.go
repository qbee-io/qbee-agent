package cmd

import (
	"context"

	"github.com/qbee-io/qbee-agent/app/agent"
)

const (
	startDisableAutoUpdateOption = "no-auto-update"
	startOnceOption              = "run-once"
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
		{
			Name:  startOnceOption,
			Short: "1",
			Help:  "Run once.",
			Flag:  "true",
		},
	},

	Target: func(opts Options) error {
		disableAutoUpdate := opts[startDisableAutoUpdateOption] == "true"
		runOnce := opts[startOnceOption] == "true"

		ctx := context.Background()

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		if runOnce {
			return agent.RunOnce(ctx, cfg)
		}

		if disableAutoUpdate || !cfg.AutoUpdate {
			return agent.Start(ctx, cfg)
		}

		return agent.StartWithAutoUpdate(ctx, cfg)
	},
}
