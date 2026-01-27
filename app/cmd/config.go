// Copyright 2023 qbee.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.qbee.io/agent/app/agent"
	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils"
	"go.qbee.io/agent/app/utils/cmd"
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

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		ctx := context.WithValue(context.Background(), utils.ContextKeyElevationCommand, cfg.ElevationCommand)

		var deviceAgent *agent.Agent

		if fromFile != "" {
			deviceAgent, err = agent.NewWithoutCredentials(cfg)
		} else {
			deviceAgent, err = agent.New(cfg)
		}

		if err != nil {
			return fmt.Errorf("error initializing the agent: %w", err)
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

		deviceAgent.Configuration.UpdateSettings(configurationData)

		if reportToConsole {
			deviceAgent.Configuration.EnableConsoleReporting()
		}

		return deviceAgent.Configuration.Execute(ctx, configurationData)
	},
}
