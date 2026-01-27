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

	"go.qbee.io/agent/app/agent"
	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/utils"
	"go.qbee.io/agent/app/utils/cmd"
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

		ctxWithElevationCommand := context.WithValue(ctx, utils.ContextKeyElevationCommand, cfg.ElevationCommand)

		if cfg.BootstrapKey != "" {
			log.Infof("Found bootstrap key, bootstrapping device.")
			if err := agent.Bootstrap(ctxWithElevationCommand, cfg); err != nil {
				return err
			}
		}

		if runOnce {
			return agent.RunOnce(ctxWithElevationCommand, cfg)
		}

		return agent.Start(ctxWithElevationCommand, cfg)
	},
}
