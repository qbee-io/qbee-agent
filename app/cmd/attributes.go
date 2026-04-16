// Copyright 2026 qbee.io
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
	"strings"

	"go.qbee.io/agent/app/agent"
	"go.qbee.io/agent/app/attributes"
	"go.qbee.io/agent/app/utils/cmd"
)

const (
	attributesShellOption = "shell"
	attributesKeyValueArg = "attributes"
)

var attributesCommand = cmd.Command{
	Description: "Manage device attributes.",
	Options:     []cmd.Option{},
	SubCommands: map[string]cmd.Command{
		"get": attributesGetCommand,
		"set": attributesSetCommand,
	},
}

var attributesGetCommand = cmd.Command{
	Description: "Get device attributes in JSON format. All attributes by default, but filters can be applied by specifying keys as arguments (e.g. device_name custom.env).",
	Options: []cmd.Option{
		{
			Name:  attributesShellOption,
			Short: "s",
			Help:  "Output shell variable formatted lines instead of JSON (e.g. QBEE_ATTRIBUTE_DEVICE_NAME=...)",
			Flag:  "true",
		},
	},
	Target: func(opts cmd.Options, args ...string) error {
		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		deviceAgent, err := agent.New(cfg)
		if err != nil {
			return fmt.Errorf("error initializing the agent: %w", err)
		}

		deviceAttrs, err := deviceAgent.Attributes.Get(context.Background())
		if err != nil {
			return err
		}

		if len(args) > 0 {
			deviceAttrs = deviceAttrs.Filter(args)
		}

		if opts[attributesShellOption] == "true" {
			for _, line := range deviceAttrs.ShellLines() {
				fmt.Println(line)
			}
			return nil
		}
		return json.NewEncoder(os.Stdout).Encode(deviceAttrs)
	},
}

var attributesSetCommand = cmd.Command{
	Description: "Set device attributes. Empty or null values delete the attribute. Default input is JSON payload",
	Options: []cmd.Option{
		{
			Name:  attributesKeyValueArg,
			Short: "a",
			Help:  `Attributes as a commma-separated list of key=value pairs (e.g. "device_name=mydevice,custom.env=prod"). Keys must be valid identifiers or start with "custom.".`,
		},
	},
	Target: func(opts cmd.Options, args ...string) error {
		attrs, err := parseAttributesInput(opts, args)
		if err != nil {
			return err
		}

		cfg, err := loadConfig(opts)
		if err != nil {
			return err
		}

		deviceAgent, err := agent.New(cfg)
		if err != nil {
			return fmt.Errorf("error initializing the agent: %w", err)
		}

		return deviceAgent.Attributes.Set(context.Background(), attrs)
	},
}

// parseAttributesInput parses attributes from either --json option or key=value positional arguments.
func parseAttributesInput(opts cmd.Options, args []string) (*attributes.DeviceAttributes, error) {
	if keyValuePars, ok := opts[attributesKeyValueArg]; ok {
		args := strings.Split(keyValuePars, ",")
		return attributes.ParseKeyValueArgs(args)
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("no JSON payload provided")
	}

	var attrs attributes.DeviceAttributes
	if err := json.Unmarshal([]byte(args[0]), &attrs); err != nil {
		return nil, fmt.Errorf("invalid JSON attributes: %w", err)
	}

	return &attrs, nil
}
