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

	"go.qbee.io/agent/app/agent"
	"go.qbee.io/agent/app/attributes"
	"go.qbee.io/agent/app/utils/cmd"
)

const (
	attributesJSONOption  = "json"
	attributesShellOption = "shell"
)

const defaultAttributesFormat = "json"

var attributesCommand = cmd.Command{
	Description: "Manage device attributes.",
	Options:     []cmd.Option{},
	SubCommands: map[string]cmd.Command{
		"get": attributesGetCommand,
		"set": attributesSetCommand,
	},
}

var attributesGetCommand = cmd.Command{
	Description: "Get device attributes.",
	Options: []cmd.Option{
		{
			Name:  attributesJSONOption,
			Short: "j",
			Help:  "Output json (default).",
			Flag:  "true",
		},
		{
			Name:  attributesShellOption,
			Short: "s",
			Help:  "Output shell variables.",
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

		format := defaultAttributesFormat
		if opts[attributesShellOption] == "true" {
			format = "shell"
		}

		// override to json if both options are set, since it's more structured and less error-prone for scripting
		if opts[attributesJSONOption] == "true" {
			format = "json"
		}

		switch format {
		case "json":
			if len(args) > 0 {
				return json.NewEncoder(os.Stdout).Encode(deviceAttrs.Filter(args))
			}
			return json.NewEncoder(os.Stdout).Encode(deviceAttrs)
		case "shell":
			if len(args) == 0 {
				for _, line := range deviceAttrs.ShellLines() {
					fmt.Println(line)
				}
				return nil
			}
			for _, key := range args {
				if v, ok := deviceAttrs.GetValue(key); ok {
					fmt.Printf("%s=%q\n", attributes.ToShellVarName(key), v)
				}
			}
			return nil

		default:
			return fmt.Errorf("unsupported format %q, use json or shell", format)
		}
	},
}

var attributesSetCommand = cmd.Command{
	Description: "Set device attributes. Empty or null values delete the attribute.",
	Options: []cmd.Option{
		{
			Name:  attributesJSONOption,
			Short: "j",
			Help:  `Attributes as a JSON array, e.g. '{"device_name":"mydevie","custom":{"mykey":"myvalue"}}'. Use null to delete an attribute.`,
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
	if jsonInput, ok := opts[attributesJSONOption]; ok {
		var attrs attributes.DeviceAttributes

		if err := json.Unmarshal([]byte(jsonInput), &attrs); err != nil {
			return nil, fmt.Errorf("invalid JSON attributes: %w", err)
		}

		return &attrs, nil
	}

	return attributes.ParseKeyValueArgs(args)
}
