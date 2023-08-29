package cmd

import (
	"context"

	"qbee.io/platform/utils/cmd"

	"github.com/qbee-io/qbee-agent/app/agent"
)

var updateCommand = cmd.Command{
	Description: "Update the agent.",
	Target: func(opts cmd.Options) error {
		ctx := context.Background()

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		return agent.Update(ctx, cfg)
	},
}
