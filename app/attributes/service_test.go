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

package attributes

import (
	"encoding/json"
	"sort"
	"testing"

	"go.qbee.io/agent/app/utils/assert"
)

func TestToShellVarName(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{key: "device_name", want: "QBEE_ATTRIBUTE_DEVICE_NAME"},
		{key: "longitude", want: "QBEE_ATTRIBUTE_LONGITUDE"},
		{key: "latitude", want: "QBEE_ATTRIBUTE_LATITUDE"},
		{key: "custom.foo", want: "QBEE_ATTRIBUTE_CUSTOM_FOO"},
		{key: "custom.foo_bar", want: "QBEE_ATTRIBUTE_CUSTOM_FOO_BAR"},
		{key: "custom.foo.bar", want: "QBEE_ATTRIBUTE_CUSTOM_FOO_BAR"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := ToShellVarName(tt.key)
			if got != tt.want {
				t.Errorf("ToShellVarName(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// TestDeviceAttributesShellLines verifies that ShellLines produces correctly named shell variables.
func TestDeviceAttributesShellLines(t *testing.T) {
	d := &DeviceAttributes{
		DeviceName: "qbee-dev-1",
		Longitude:  "12.34",
		Latitude:   "12.34",
		Custom:     map[string]string{"mykey": "myvalue"},
	}

	lines := d.ShellLines()
	sort.Strings(lines)

	want := []string{
		`QBEE_ATTRIBUTE_DEVICE_NAME="qbee-dev-1"`,
		`QBEE_ATTRIBUTE_LONGITUDE="12.34"`,
		`QBEE_ATTRIBUTE_LATITUDE="12.34"`,
		`QBEE_ATTRIBUTE_CUSTOM_MYKEY="myvalue"`,
	}
	sort.Strings(want)

	assert.Equal(t, lines, want)
}

func TestParseKeyValueArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    *DeviceAttributes
		wantErr bool
	}{
		{
			name: "device_name key=value",
			args: []string{"device_name=mydevice"},
			want: &DeviceAttributes{DeviceName: "mydevice"},
		},
		{
			name: "multiple allowed keys",
			args: []string{"longitude=12.34", "latitude=56.78"},
			want: &DeviceAttributes{
				Longitude: "12.34",
				Latitude:  "56.78",
			},
		},
		{
			name: "custom key",
			args: []string{"custom.env=production"},
			want: &DeviceAttributes{Custom: map[string]string{"env": "production"}},
		},
		{
			name: "value with equals sign",
			args: []string{"custom.url=http://example.com?a=1"},
			want: &DeviceAttributes{Custom: map[string]string{"url": "http://example.com?a=1"}},
		},
		{
			name: "empty value signals deletion",
			args: []string{"device_name="},
			want: &DeviceAttributes{DeviceName: ""},
		},
		{
			name:    "missing equals sign",
			args:    []string{"device_name"},
			wantErr: true,
		},
		{
			name: "invalid key rejected",
			args: []string{"hostname=foo"},
			want: &DeviceAttributes{},
		},
		{
			name: "empty args",
			args: []string{},
			want: &DeviceAttributes{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseKeyValueArgs(tt.args)

			if err != nil {
				if !tt.wantErr {
					t.Errorf("ParseKeyValueArgs() error = %v, wantErr %v", err, tt.wantErr)

				}
				return // error was expected, test passed
			}

			if tt.wantErr {
				t.Errorf("ParseKeyValueArgs() expected error but got nil")
				return
			}

			assert.Equal(t, got, tt.want)
		})
	}
}

func TestFilter(t *testing.T) {
	attrs := &DeviceAttributes{
		DeviceName: "mydevice",
		Longitude:  "12.34",
		Latitude:   "56.78",
		Custom:     map[string]string{"env": "prod", "version": "1.0"},
	}

	tests := []struct {
		name       string
		keys       []string
		want       *DeviceAttributes
		json       []byte
		shellLines []string
	}{
		{
			name: "predefined attributes",
			keys: []string{"device_name", "longitude"},
			want: &DeviceAttributes{
				DeviceName: "mydevice",
				Longitude:  "12.34",
			},
			shellLines: []string{
				`QBEE_ATTRIBUTE_DEVICE_NAME="mydevice"`,
				`QBEE_ATTRIBUTE_LONGITUDE="12.34"`,
			},
			json: []byte(`{"device_name":"mydevice","longitude":"12.34"}`),
		},
		{
			name: "custom attributes",
			keys: []string{"custom.env", "custom.version"},
			want: &DeviceAttributes{
				Custom: map[string]string{
					"env":     "prod",
					"version": "1.0",
				},
			},
			shellLines: []string{
				`QBEE_ATTRIBUTE_CUSTOM_ENV="prod"`,
				`QBEE_ATTRIBUTE_CUSTOM_VERSION="1.0"`,
			},
			json: []byte(`{"custom":{"env":"prod","version":"1.0"}}`),
		},
		{
			name: "mixed predefined and custom",
			keys: []string{"latitude", "custom.env"},
			want: &DeviceAttributes{
				Latitude: "56.78",
				Custom: map[string]string{
					"env": "prod",
				},
			},
			shellLines: []string{
				`QBEE_ATTRIBUTE_LATITUDE="56.78"`,
				`QBEE_ATTRIBUTE_CUSTOM_ENV="prod"`,
			},
			json: []byte(`{"latitude":"56.78","custom":{"env":"prod"}}`),
		},
		{
			name: "unknown keys are ignored",
			keys: []string{"device_name", "unknown_key", "custom.unknown"},
			want: &DeviceAttributes{
				DeviceName: "mydevice",
			},
			shellLines: []string{
				`QBEE_ATTRIBUTE_DEVICE_NAME="mydevice"`,
			},
			json: []byte(`{"device_name":"mydevice"}`),
		},
		{
			name:       "empty keys",
			keys:       []string{},
			want:       &DeviceAttributes{},
			shellLines: []string{},
			json:       []byte(`{}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := attrs.Filter(tt.keys)
			assert.Equal(t, got, tt.want)

			if tt.shellLines != nil {
				lines := got.ShellLines()
				sort.Strings(lines)
				sort.Strings(tt.shellLines)
				assert.Equal(t, lines, tt.shellLines)
			}

			if tt.json != nil {
				jsonData, err := json.Marshal(got)
				if err != nil {
					t.Fatalf("MarshalJSON() error: %v", err)
				}
				assert.Equal(t, jsonData, tt.json)
			}
		})
	}
}
