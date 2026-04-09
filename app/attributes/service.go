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

// allowedKeys lists the attribute keys that are not custom (i.e. predefined).
var allowedKeys = map[string]bool{
	"device_name": true,
	"longitude":   true,
	"latitude":    true,
}

// ValidateKey returns an error if key is not one of the allowed attribute keys.
// Allowed keys are: device_name, longitude, latitude, and custom.* (any key with a "custom." prefix).
func ValidateKey(key string) error {
	if allowedKeys[key] {
		return nil
	}

	if strings.HasPrefix(key, "custom.") && len(key) > len("custom.") {
		return nil
	}

	return fmt.Errorf("invalid attribute key %q: allowed keys are device_name, longitude, latitude, and custom.*", key)
}

// ToShellVarName converts an attribute key to its QBEE_ATTRIBUTE_ shell variable name.
// Custom attributes (custom.*) get the QBEE_ATTRIBUTE_CUSTOM_ prefix.
// All other allowed attributes get the QBEE_ATTRIBUTE_ prefix.
// The key is uppercased and dots/dashes are replaced with underscores.
func ToShellVarName(key string) string {
	if strings.HasPrefix(key, "custom.") {
		suffix := strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(key[len("custom."):]))
		return "QBEE_ATTRIBUTE_CUSTOM_" + suffix
	}

	return "QBEE_ATTRIBUTE_" + strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(key))
}

// Attribute represents a single key-value attribute.
// Value is a pointer to allow null values, which will delete the attribute.
type Attribute struct {
	Key   string  `json:"key"`
	Value *string `json:"value"`
}

// Attributes is a slice of Attribute.
type Attributes []Attribute

// ParseKeyValueArgs parses "key=value" strings into Attributes.
// An empty value (e.g. "key=") is treated as an empty string and will delete the attribute.
func ParseKeyValueArgs(args []string) (Attributes, error) {
	attrs := make(Attributes, 0, len(args))

	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key=value pair: %q", arg)
		}

		if err := ValidateKey(parts[0]); err != nil {
			return nil, err
		}

		value := parts[1]
		attrs = append(attrs, Attribute{Key: parts[0], Value: &value})
	}

	return attrs, nil
}

// ParseJSONArgs parses a JSON list of attribute objects into Attributes.
// A null value will delete the attribute.
func ParseJSONArgs(data string) (Attributes, error) {
	var attrs Attributes

	if err := json.Unmarshal([]byte(data), &attrs); err != nil {
		return nil, fmt.Errorf("invalid JSON attributes: %w", err)
	}

	for _, attr := range attrs {
		if err := ValidateKey(attr.Key); err != nil {
			return nil, err
		}
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
// Attributes with a null or empty value will be deleted.
func (srv *Service) Set(ctx context.Context, attrs Attributes) error {
	if err := srv.api.Post(ctx, attributesAPIPath, attrs, nil); err != nil {
		return fmt.Errorf("error setting attributes: %w", err)
	}

	return nil
}
