package cmd

import (
	"context"

	"qbee.io/platform/utils/cmd"

	"github.com/qbee-io/qbee-agent/app/agent"
	"github.com/qbee-io/qbee-agent/app/log"
)

const (
	startOnceOption = "run-once"
)

var startCommand = cmd.Command{
	Description: "Start the agent process.",

	Options: []cmd.Option{
		{
			Name:  startOnceOption,
			Short: "1",
			Help:  "Run once.",
			Flag:  "true",
		},
	},

	Target: func(opts cmd.Options) error {
		runOnce := opts[startOnceOption] == "true"

		ctx := context.Background()

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		if cfg.BootstrapKey != "" {
			log.Infof("Found bootstrap key, bootstrapping device.")
			if err := agent.Bootstrap(ctx, cfg); err != nil {
				return err
			}
		}

		if runOnce {
			return agent.RunOnce(ctx, cfg)
		}

		return agent.Start(ctx, cfg)
	},
}
