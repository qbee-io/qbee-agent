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

package configuration

import (
	"testing"

	"go.qbee.io/agent/app/utils/assert"
)

func TestCommittedConfig_SecretsList(t *testing.T) {
	tests := []struct {
		name     string
		config   CommittedConfig
		expected []string
	}{
		{
			name:     "no bundles",
			config:   CommittedConfig{},
			expected: nil,
		},
		{
			name: "parameters secrets",
			config: CommittedConfig{
				BundleData: BundleData{
					Parameters: &ParametersBundle{
						Secrets: []Parameter{
							{Key: "db_password", Value: "param-secret-1"},
							{Key: "api_token", Value: "param-secret-2"},
						},
					},
				},
			},
			expected: []string{"param-secret-1", "param-secret-2"},
		},
		{
			name: "docker compose registry secrets",
			config: CommittedConfig{
				BundleData: BundleData{
					DockerCompose: &DockerComposeBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "compose-secret"},
						},
					},
				},
			},
			expected: []string{"compose-secret"},
		},
		{
			name: "docker containers registry secrets",
			config: CommittedConfig{
				BundleData: BundleData{
					DockerContainers: &DockerContainersBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "docker-secret"},
						},
					},
				},
			},
			expected: []string{"docker-secret"},
		},
		{
			name: "podman containers registry secrets",
			config: CommittedConfig{
				BundleData: BundleData{
					PodmanContainers: &PodmanContainerBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "podman-secret"},
						},
					},
				},
			},
			expected: []string{"podman-secret"},
		},
		{
			name: "secrets from all bundles",
			config: CommittedConfig{
				BundleData: BundleData{
					Parameters: &ParametersBundle{
						Secrets: []Parameter{
							{Key: "db_password", Value: "param-secret"},
						},
					},
					DockerCompose: &DockerComposeBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "compose-secret"},
						},
					},
					DockerContainers: &DockerContainersBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "docker-secret"},
						},
					},
					PodmanContainers: &PodmanContainerBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "podman-secret"},
						},
					},
				},
			},
			expected: []string{"param-secret", "compose-secret", "docker-secret", "podman-secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.config.SecretsList(), tt.expected)
		})
	}
}
