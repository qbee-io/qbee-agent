package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/qbee-io/qbee-agent/app/agent"
	"github.com/qbee-io/qbee-agent/app/configuration"
)

const (
	configDryRunOption = "dry-run"
)

var configCommand = Command{
	Description: "Execute device configuration.",
	Options: []Option{
		{
			Name:  configDryRunOption,
			Short: "d",
			Help:  "Don't apply configuration. Just dump current configuration as JSON to standard output.",
			Flag:  "true",
		},
	},
	Target: func(opts Options) error {
		dryRun := opts[inventoryDryRunOption] == "true"

		ctx := context.Background()

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		var deviceAgent *agent.Agent
		if deviceAgent, err = agent.New(cfg); err != nil {
			return fmt.Errorf("error initializing the agent: %w", err)
		}

		var configurationData *configuration.CommittedConfig
		if configurationData, err = deviceAgent.Configuration.Get(ctx); err != nil {
			return err
		}

		if dryRun {
			return json.NewEncoder(os.Stdout).Encode(configurationData)
		}

		return deviceAgent.Configuration.Execute(ctx, configurationData)
	},
}
