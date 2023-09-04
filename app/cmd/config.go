package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"qbee.io/platform/utils/cmd"

	"github.com/qbee-io/qbee-agent/app/agent"
	"github.com/qbee-io/qbee-agent/app/configuration"
)

const (
	configFromFileOption        = "from-file"
	configDryRunOption          = "dry-run"
	configReportToConsoleOption = "report-to-console"
)

var configCommand = cmd.Command{
	Description: "Execute device configuration.",
	Options: []cmd.Option{
		{
			Name:  configFromFileOption,
			Short: "f",
			Help:  "Apply configuration from provided file.",
		},
		{
			Name:  configReportToConsoleOption,
			Short: "r",
			Help:  "Print configuration reports to console.",
			Flag:  "true",
		},
		{
			Name:  configDryRunOption,
			Short: "d",
			Help:  "Don't apply configuration. Just dump current configuration as JSON to standard output.",
			Flag:  "true",
		},
	},
	Target: func(opts cmd.Options) error {
		dryRun := opts[configDryRunOption] == "true"
		fromFile := opts[configFromFileOption]
		reportToConsole := opts[configReportToConsoleOption] == "true"

		ctx := context.Background()

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		var deviceAgent *agent.Agent

		if fromFile != "" {
			deviceAgent, err = agent.NewWithoutCredentials(cfg)
		} else {
			deviceAgent, err = agent.New(cfg)
		}

		if err != nil {
			return fmt.Errorf("error initializing the agent: %w", err)
		}

		if reportToConsole {
			deviceAgent.Configuration.EnableConsoleReporting()
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
