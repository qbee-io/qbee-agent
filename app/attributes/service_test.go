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
	"reflect"
	"testing"
)

func strPtr(s string) *string { return &s }

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "device_name", key: "device_name"},
		{name: "longitude", key: "longitude"},
		{name: "latitude", key: "latitude"},
		{name: "custom with suffix", key: "custom.foo"},
		{name: "custom with dot suffix", key: "custom.foo.bar"},
		{name: "custom prefix only is invalid", key: "custom.", wantErr: true},
		{name: "arbitrary key is invalid", key: "hostname", wantErr: true},
		{name: "empty key is invalid", key: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

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

func TestParseKeyValueArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Attributes
		wantErr bool
	}{
		{
			name: "device_name key=value",
			args: []string{"device_name=mydevice"},
			want: Attributes{{Key: "device_name", Value: strPtr("mydevice")}},
		},
		{
			name: "multiple allowed keys",
			args: []string{"longitude=12.34", "latitude=56.78"},
			want: Attributes{
				{Key: "longitude", Value: strPtr("12.34")},
				{Key: "latitude", Value: strPtr("56.78")},
			},
		},
		{
			name: "custom key",
			args: []string{"custom.env=production"},
			want: Attributes{{Key: "custom.env", Value: strPtr("production")}},
		},
		{
			name: "value with equals sign",
			args: []string{"custom.url=http://example.com?a=1"},
			want: Attributes{{Key: "custom.url", Value: strPtr("http://example.com?a=1")}},
		},
		{
			name: "empty value deletes attribute",
			args: []string{"device_name="},
			want: Attributes{{Key: "device_name", Value: strPtr("")}},
		},
		{
			name:    "missing equals sign",
			args:    []string{"device_name"},
			wantErr: true,
		},
		{
			name:    "invalid key rejected",
			args:    []string{"hostname=foo"},
			wantErr: true,
		},
		{
			name: "empty args",
			args: []string{},
			want: Attributes{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseKeyValueArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKeyValueArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseKeyValueArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseJSONArgs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Attributes
		wantErr bool
	}{
		{
			name:  "device_name attribute",
			input: `[{"key":"device_name","value":"mydevice"}]`,
			want:  Attributes{{Key: "device_name", Value: strPtr("mydevice")}},
		},
		{
			name:  "null value deletes attribute",
			input: `[{"key":"longitude","value":null}]`,
			want:  Attributes{{Key: "longitude", Value: nil}},
		},
		{
			name:  "custom attribute",
			input: `[{"key":"custom.env","value":"prod"}]`,
			want:  Attributes{{Key: "custom.env", Value: strPtr("prod")}},
		},
		{
			name:  "multiple attributes",
			input: `[{"key":"longitude","value":"12.34"},{"key":"custom.env","value":"prod"}]`,
			want: Attributes{
				{Key: "longitude", Value: strPtr("12.34")},
				{Key: "custom.env", Value: strPtr("prod")},
			},
		},
		{
			name:  "empty array",
			input: `[]`,
			want:  Attributes{},
		},
		{
			name:    "invalid key rejected",
			input:   `[{"key":"hostname","value":"foo"}]`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			input:   `not json`,
			wantErr: true,
		},
		{
			name:    "json object instead of array",
			input:   `{"key":"device_name","value":"foo"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseJSONArgs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSONArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseJSONArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
