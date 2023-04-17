package cmd

import (
	"context"

	"github.com/qbee-io/qbee-agent/app/agent"
)

var updateCommand = Command{
	Description: "Update the agent.",
	Target: func(opts Options) error {
		ctx := context.Background()

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		return agent.Update(ctx, cfg)
	},
}
