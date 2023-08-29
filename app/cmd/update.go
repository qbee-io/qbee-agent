package cmd

import (
	"context"

	"qbee.io/platform/utils/flags"

	"github.com/qbee-io/qbee-agent/app/agent"
)

var updateCommand = flags.Command{
	Description: "Update the agent.",
	Target: func(opts flags.Options) error {
		ctx := context.Background()

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		return agent.Update(ctx, cfg)
	},
}
