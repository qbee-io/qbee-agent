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
	"go.qbee.io/agent/app/agent"
	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/utils/cmd"
)

const (
	mainConfigDirOption = "config-dir"
	mainStateDirOption  = "state-dir"
	mainLogLevel        = "log-level"
)

const (
	defaultConfigDir = "/etc/qbee"
	defaultStateDir  = "/var/lib/qbee"
)

// Main is the main command of the agent.
var Main = cmd.Command{
	Description: "Qbee Agent Command-Line Tool",
	Options: []cmd.Option{
		{
			Name:    mainConfigDirOption,
			Short:   "c",
			Help:    "Configuration directory.",
			Default: defaultConfigDir,
		},
		{
			Name:    mainStateDirOption,
			Short:   "s",
			Help:    "State directory.",
			Default: defaultStateDir,
		},
		{
			Name:    mainLogLevel,
			Short:   "l",
			Help:    "Logging level: DEBUG, INFO, WARNING or ERROR.",
			Default: "INFO",
		},
	},
	SubCommands: map[string]cmd.Command{
		"bootstrap": bootstrapCommand,
		"config":    configCommand,
		"inventory": inventoryCommand,
		"start":     startCommand,
		"update":    updateCommand,
		"version":   versionCommand,
	},
}

// loadConfig is a helper method to load agent's config based on provided command-line options.
func loadConfig(opts cmd.Options) (*agent.Config, error) {
	switch opts[mainLogLevel] {
	case "DEBUG":
		log.SetLevel(log.DEBUG)
	case "INFO":
		log.SetLevel(log.INFO)
	case "WARNING":
		log.SetLevel(log.WARNING)
	case "ERROR":
		log.SetLevel(log.ERROR)
	}

	return agent.LoadConfig(opts[mainConfigDirOption], opts[mainStateDirOption])
}
