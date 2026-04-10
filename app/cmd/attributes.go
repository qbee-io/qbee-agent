// Copyright 2024 qbee.io
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
	attributesFormatOption = "format"
	attributesKeyOption    = "key"
	attributesJSONOption   = "json"
)

var attributesCommand = cmd.Command{
	Description: "Manage device attributes.",
	SubCommands: map[string]cmd.Command{
		"get": attributesGetCommand,
		"set": attributesSetCommand,
	},
}

var attributesGetCommand = cmd.Command{
	Description: "Get device attributes.",
	Options: []cmd.Option{
		{
			Name:    attributesFormatOption,
			Short:   "f",
			Help:    "Output format: json or shell.",
			Default: "json",
		},
		{
			Name:  attributesKeyOption,
			Short: "k",
			Help:  "Filter output to a specific attribute key. Can be repeated.",
			Multi: true,
		},
	},
	Target: func(opts cmd.Options) error {
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

		keys := opts.MultiValues(attributesKeyOption)

		switch opts[attributesFormatOption] {
		case "json":
			if len(keys) > 0 {
				return json.NewEncoder(os.Stdout).Encode(deviceAttrs.FilterToMap(keys))
			}
			return json.NewEncoder(os.Stdout).Encode(deviceAttrs)
		case "shell":
			if len(keys) > 0 {
				for _, key := range keys {
					if v, ok := deviceAttrs.GetValue(key); ok {
						fmt.Printf("%s=%q\n", attributes.ToShellVarName(key), v)
					}
				}
				return nil
			}
			for _, line := range deviceAttrs.ShellLines() {
				fmt.Println(line)
			}
			return nil
		default:
			return fmt.Errorf("unsupported format %q, use json or shell", opts[attributesFormatOption])
		}
	},
}

var attributesSetCommand = cmd.Command{
	Description: "Set device attributes. Empty or null values delete the attribute.",
	Options: []cmd.Option{
		{
			Name:  attributesJSONOption,
			Short: "j",
			Help:  `Attributes as a JSON array, e.g. '[{"key":"key1","value":"value1"}]'. Use null to delete an attribute.`,
		},
	},
	Target: func(opts cmd.Options) error {
		attrs, err := parseAttributesInput(opts)
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
func parseAttributesInput(opts cmd.Options) (attributes.Attributes, error) {
	if jsonInput, ok := opts[attributesJSONOption]; ok {
		return attributes.ParseJSONArgs(jsonInput)
	}

	return attributes.ParseKeyValueArgs(opts.RemainingArgs())
}
