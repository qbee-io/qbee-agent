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

package attributes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.qbee.io/agent/app/api"
)

const attributesAPIPath = "/v1/org/device/auth/attributes"

// Attribute represents a single key-value attribute.
type Attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Attributes is a slice of Attribute.
type Attributes []Attribute

// ParseKeyValueArgs parses "key=value" strings into Attributes.
func ParseKeyValueArgs(args []string) (Attributes, error) {
	attrs := make(Attributes, 0, len(args))

	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key=value pair: %q", arg)
		}

		attrs = append(attrs, Attribute{Key: parts[0], Value: parts[1]})
	}

	return attrs, nil
}

// ParseJSONArgs parses a JSON list of attribute objects into Attributes.
func ParseJSONArgs(data string) (Attributes, error) {
	var attrs Attributes

	if err := json.Unmarshal([]byte(data), &attrs); err != nil {
		return nil, fmt.Errorf("invalid JSON attributes: %w", err)
	}

	return attrs, nil
}

// Service provides methods for managing device attributes.
type Service struct {
	api *api.Client
}

// New returns a new attributes Service.
func New(apiClient *api.Client) *Service {
	return &Service{api: apiClient}
}

// Get retrieves current device attributes from the device hub.
func (srv *Service) Get(ctx context.Context) (Attributes, error) {
	var attrs Attributes

	if err := srv.api.Get(ctx, attributesAPIPath, &attrs); err != nil {
		return nil, fmt.Errorf("error getting attributes: %w", err)
	}

	return attrs, nil
}

// Set replaces all device attributes.
func (srv *Service) Set(ctx context.Context, attrs Attributes) error {
	if err := srv.api.Post(ctx, attributesAPIPath, attrs, nil); err != nil {
		return fmt.Errorf("error setting attributes: %w", err)
	}

	return nil
}

// Update merges provided attributes with existing device attributes.
func (srv *Service) Update(ctx context.Context, attrs Attributes) error {
	if err := srv.api.Put(ctx, attributesAPIPath, attrs, nil); err != nil {
		return fmt.Errorf("error updating attributes: %w", err)
	}

	return nil
}
