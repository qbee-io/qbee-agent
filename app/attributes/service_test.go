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

func TestParseKeyValueArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Attributes
		wantErr bool
	}{
		{
			name: "single key=value",
			args: []string{"key1=value1"},
			want: Attributes{{Key: "key1", Value: "value1"}},
		},
		{
			name: "multiple key=value",
			args: []string{"key1=value1", "key2=value2"},
			want: Attributes{
				{Key: "key1", Value: "value1"},
				{Key: "key2", Value: "value2"},
			},
		},
		{
			name: "value with equals sign",
			args: []string{"key1=val=ue1"},
			want: Attributes{{Key: "key1", Value: "val=ue1"}},
		},
		{
			name:    "missing equals sign",
			args:    []string{"key1"},
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
			name:  "single attribute",
			input: `[{"key":"key1","value":"value1"}]`,
			want:  Attributes{{Key: "key1", Value: "value1"}},
		},
		{
			name:  "multiple attributes",
			input: `[{"key":"key1","value":"value1"},{"key":"key2","value":"value2"}]`,
			want: Attributes{
				{Key: "key1", Value: "value1"},
				{Key: "key2", Value: "value2"},
			},
		},
		{
			name:  "empty array",
			input: `[]`,
			want:  Attributes{},
		},
		{
			name:    "invalid json",
			input:   `not json`,
			wantErr: true,
		},
		{
			name:    "json object instead of array",
			input:   `{"key":"key1","value":"value1"}`,
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
