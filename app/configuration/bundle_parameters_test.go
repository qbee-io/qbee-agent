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

package configuration

import (
	"context"
	"net/url"
	"os"
	"testing"

	"go.qbee.io/agent/app/inventory"
	"go.qbee.io/agent/app/software"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

// mockURLSigner is a mock implementation of the URLSigner interface.
type mockURLSigner struct{}

// SignURL returns a signed URL for the given source URL.
func (m *mockURLSigner) SignURL(src string) (string, error) {
	parsedURL, err := url.Parse(src)
	if err != nil {
		return "", err
	}

	parsedURL.Scheme = "https"
	parsedURL.Host = "example.com"
	parsedURL.RawQuery = "signed=true"

	return parsedURL.String(), nil
}

func Test_resolveParameters(t *testing.T) {
	hostname, err := os.Hostname()
	assert.NoError(t, err)

	pkgArch, err := software.DefaultPackageManager.PackageArchitecture(context.Background())
	assert.NoError(t, err)

	pkgType := software.DefaultPackageManager.Type()

	systemInventory, err := inventory.CollectSystemInventory(false)
	assert.NoError(t, err)

	invSystem := systemInventory.System

	tests := []struct {
		name       string
		parameters []Parameter
		secrets    []Parameter
		value      string
		want       string
	}{
		{
			name:       "no parameters",
			parameters: []Parameter{},
			value:      "example $(key)",
			want:       "example $(key)",
		},
		{
			name: "has parameter",
			parameters: []Parameter{
				{Key: "key", Value: "test-value"},
			},
			value: "example $(key)",
			want:  "example test-value",
		},
		{
			name: "has secret",
			secrets: []Parameter{
				{Key: "secret", Value: "test-secret"},
			},
			value: "example $(secret)",
			want:  "example test-secret",
		},
		{
			name: "match the same key twice",
			parameters: []Parameter{
				{Key: "key", Value: "test-value"},
			},
			value: "example $(key) - $(key)",
			want:  "example test-value - test-value",
		},
		{
			name: "match more than one key",
			parameters: []Parameter{
				{Key: "key1", Value: "test-value-1"},
				{Key: "key2", Value: "test-value-2"},
			},
			value: "example $(key1) - $(key2)",
			want:  "example test-value-1 - test-value-2",
		},
		{
			name: "unclosed key tag",
			parameters: []Parameter{
				{Key: "key", Value: "test-value"},
			},
			value: "example $(key remaining text",
			want:  "example $(key remaining text",
		},
		{
			name: "ending with $",
			parameters: []Parameter{
				{Key: "key", Value: "test-value"},
			},
			value: "example $",
			want:  "example $",
		},
		{
			name: "ending with $(",
			parameters: []Parameter{
				{Key: "key", Value: "test-value"},
			},
			value: "example $(",
			want:  "example $(",
		},
		{
			name:       "system variable",
			parameters: []Parameter{},
			value:      "example $(sys.host)",
			want:       "example " + hostname,
		},
		{
			name:       "more than one system variable",
			parameters: []Parameter{},
			value:      "example $(sys.host) $(sys.pkg_arch) $(sys.pkg_type)",
			want:       "example " + hostname + " " + pkgArch + " " + string(pkgType),
		},
		{
			name:       "inventory related sys variables",
			parameters: []Parameter{},
			value:      "example $(sys.os) $(sys.os_type) $(sys.flavor) $(sys.boot_time)",
			want:       "example " + invSystem.OS + " " + invSystem.OSType + " " + invSystem.Flavor + " " + invSystem.BootTime,
		},
		{
			name:       "signed file manager url",
			parameters: []Parameter{},
			value:      "example $(file://test.cfg)",
			want:       "example https://example.com/v1/org/device/public/files/test.cfg?signed=true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parametersBundle := ParametersBundle{
				Parameters: tt.parameters,
				Secrets:    tt.secrets,
			}

			ctx := parametersBundle.Context(context.Background(), new(mockURLSigner))

			got := resolveParameters(ctx, tt.value)

			assert.Equal(t, got, tt.want)
		})
	}
}

func Test_UsersWithParameters(t *testing.T) {
	r := runner.New(t)

	// commit config for the device
	cfg := CommittedConfig{
		Bundles: []string{BundleParameters, BundleUsers},
		BundleData: BundleData{
			Parameters: &ParametersBundle{
				Metadata: Metadata{Enabled: true},
				Parameters: []Parameter{
					{Key: "plain", Value: "plainUsername"},
				},
				Secrets: []Parameter{
					{Key: "secret", Value: "secretUsername"},
				},
			},
			Users: &UsersBundle{
				Metadata: Metadata{Enabled: true},
				Users: []User{
					{
						Username: "$(plain)",
						Action:   UserAdd,
					}, {
						Username: "$(secret)",
						Action:   UserAdd,
					},
				},
			},
		},
	}

	reports, _ := ExecuteTestConfigInDocker(r, cfg)

	// execute configuration bundles
	expectedReports := []string{
		"[INFO] Successfully added user 'plainUsername'",
		"[INFO] Successfully added user '********'",
	}
	assert.Equal(t, reports, expectedReports)
}
