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
	configFromFileOption = "from-file"
	configDryRunOption   = "dry-run"
)

var configCommand = Command{
	Description: "Execute device configuration.",
	Options: []Option{
		{
			Name:  configFromFileOption,
			Short: "f",
			Help:  "Apply configuration from provided file.",
		},
		{
			Name:  configDryRunOption,
			Short: "d",
			Help:  "Don't apply configuration. Just dump current configuration as JSON to standard output.",
			Flag:  "true",
		},
	},
	Target: func(opts Options) error {
		dryRun := opts[configDryRunOption] == "true"
		fromFile := opts[configFromFileOption]

		ctx := context.Background()

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		var deviceAgent *agent.Agent

		if fromFile != "" {
			deviceAgent, err = agent.NewWithoutCredentials(cfg)
			deviceAgent.Configuration.EnableConsoleReporting()
		} else {
			deviceAgent, err = agent.New(cfg)
		}

		if err != nil {
			return fmt.Errorf("error initializing the agent: %w", err)
		}

		var configurationData *configuration.CommittedConfig

		if fromFile != "" {
			configBytes, err := os.ReadFile(fromFile)
			if err != nil {
				return fmt.Errorf("cannot open local config file: %w", err)
			}

			configurationData = new(configuration.CommittedConfig)

			if err = json.Unmarshal(configBytes, configurationData); err != nil {
				return fmt.Errorf("cannot parse local config file: %w", err)
			}
		} else {
			if configurationData, err = deviceAgent.Configuration.Get(ctx); err != nil {
				return err
			}
		}

		if dryRun {
			return json.NewEncoder(os.Stdout).Encode(configurationData)
		}

		return deviceAgent.Configuration.Execute(ctx, configurationData)
	},
}
