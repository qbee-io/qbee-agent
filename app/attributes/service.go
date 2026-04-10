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
	"sort"
	"strings"

	"go.qbee.io/agent/app/api"
)

const attributesAPIPath = "/v1/org/device/auth/attributes"

// allowedKeys lists the predefined (non-custom) attribute keys.
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

// DeviceAttributes matches the flat-object JSON format used by the API:
//
//	{"device_name":"...","longitude":"...","latitude":"...","custom":{"key":"value",...}}
type DeviceAttributes struct {
	DeviceName string            `json:"device_name"`
	Longitude  string            `json:"longitude"`
	Latitude   string            `json:"latitude"`
	Custom     map[string]string `json:"custom"`
}

// ShellLines returns QBEE_ATTRIBUTE_*="value" lines suitable for shell sourcing.
// Predefined attributes are always included; custom attributes are sorted for determinism.
func (d DeviceAttributes) ShellLines() []string {
	lines := []string{
		fmt.Sprintf("%s=%q", ToShellVarName("device_name"), d.DeviceName),
		fmt.Sprintf("%s=%q", ToShellVarName("longitude"), d.Longitude),
		fmt.Sprintf("%s=%q", ToShellVarName("latitude"), d.Latitude),
	}

	keys := make([]string, 0, len(d.Custom))
	for k := range d.Custom {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%q", ToShellVarName("custom."+k), d.Custom[k]))
	}

	return lines
}

// GetValue returns the value for a given normalized attribute key (e.g. "device_name", "custom.mykey").
// Returns ("", false) if the key is not present.
func (d DeviceAttributes) GetValue(key string) (string, bool) {
	switch key {
	case "device_name":
		return d.DeviceName, true
	case "longitude":
		return d.Longitude, true
	case "latitude":
		return d.Latitude, true
	default:
		if strings.HasPrefix(key, "custom.") {
			suffix := key[len("custom."):]
			if d.Custom != nil {
				v, ok := d.Custom[suffix]
				return v, ok
			}
		}
		return "", false
	}
}

// Filter returns a map containing only the specified keys with their values, using the same
// nested structure as the full DeviceAttributes JSON (custom.* keys are nested under "custom").
// Unknown or missing keys are silently omitted.
func (d DeviceAttributes) Filter(keys []string) map[string]interface{} {
	result := make(map[string]interface{})
	var custom map[string]string

	for _, key := range keys {
		switch key {
		case "device_name":
			result["device_name"] = d.DeviceName
		case "longitude":
			result["longitude"] = d.Longitude
		case "latitude":
			result["latitude"] = d.Latitude
		default:
			if strings.HasPrefix(key, "custom.") {
				suffix := key[len("custom."):]
				if d.Custom != nil {
					if v, ok := d.Custom[suffix]; ok {
						if custom == nil {
							custom = make(map[string]string)
						}
						custom[suffix] = v
					}
				}
			}
		}
	}

	if custom != nil {
		result["custom"] = custom
	}

	return result
}

// toAPIPayload converts an Attributes slice into a map suitable for JSON-encoding and sending to the
// API. Only attributes present in the slice are included in the payload, so callers control which
// fields are touched. A nil or empty Value signals deletion (encoded as JSON null).
func (attrs Attributes) toAPIPayload() map[string]interface{} {
	payload := make(map[string]interface{})
	custom := make(map[string]interface{})
	hasCustom := false

	for _, attr := range attrs {
		// Nil or empty string → null (delete); otherwise keep the value.
		var val interface{}
		if attr.Value != nil && *attr.Value != "" {
			val = *attr.Value
		}

		switch attr.Key {
		case "device_name", "longitude", "latitude":
			payload[attr.Key] = val
		default:
			if strings.HasPrefix(attr.Key, "custom.") {
				suffix := attr.Key[len("custom."):]
				custom[suffix] = val
				hasCustom = true
			}
		}
	}

	if hasCustom {
		payload["custom"] = custom
	}

	return payload
}

// Attribute represents a single key-value attribute in the internal/CLI representation.
// Value is a pointer to allow null values, which will delete the attribute.
type Attribute struct {
	Key   string  `json:"key"`
	Value *string `json:"value"`
}

// Attributes is a slice of Attribute.
type Attributes []Attribute

// ParseKeyValueArgs parses "key=value" strings into Attributes.
// An empty value (e.g. "key=") signals deletion of that attribute.
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

// ParseJSONArgs parses a JSON array of {"key":"...","value":"..."} objects into Attributes.
// A null value signals deletion of that attribute.
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
func (srv *Service) Get(ctx context.Context) (DeviceAttributes, error) {
	var response DeviceAttributes

	if err := srv.api.Get(ctx, attributesAPIPath, &response); err != nil {
		return DeviceAttributes{}, fmt.Errorf("error getting attributes: %w", err)
	}

	return response, nil
}

// Set updates device attributes. Only the supplied attributes are sent to the API.
// An empty or null Value signals that the attribute should be deleted.
func (srv *Service) Set(ctx context.Context, attrs Attributes) error {
	payload := attrs.toAPIPayload()

	if err := srv.api.Patch(ctx, attributesAPIPath, payload, nil); err != nil {
		return fmt.Errorf("error setting attributes: %w", err)
	}

	return nil
}
