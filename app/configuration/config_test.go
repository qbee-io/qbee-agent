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
	"context"
	"testing"

	"go.qbee.io/agent/app/utils/assert"
)

func TestCommittedConfig_SecretsList(t *testing.T) {
	tests := []struct {
		name string

		// storeSecrets are placed in the resolution context, so registry passwords
		// referencing them via $(key) can be resolved.
		storeSecrets []Parameter

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
							{Key: "db_password", Value: "param-secret-longest"},
							{Key: "api_token", Value: "param-secret-mid"},
						},
					},
				},
			},
			expected: []string{"param-secret-longest", "param-secret-mid"},
		},
		{
			name: "docker compose registry secret resolved from parameter",
			storeSecrets: []Parameter{
				{Key: "compose_registry_password", Value: "resolved-compose-secret"},
			},
			config: CommittedConfig{
				BundleData: BundleData{
					DockerCompose: &DockerComposeBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "$(compose_registry_password)"},
						},
					},
				},
			},
			expected: []string{"resolved-compose-secret"},
		},
		{
			name: "docker containers registry secret resolved from parameter",
			storeSecrets: []Parameter{
				{Key: "docker_registry_password", Value: "resolved-docker-secret"},
			},
			config: CommittedConfig{
				BundleData: BundleData{
					DockerContainers: &DockerContainersBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "$(docker_registry_password)"},
						},
					},
				},
			},
			expected: []string{"resolved-docker-secret"},
		},
		{
			name: "podman containers registry secret resolved from parameter",
			storeSecrets: []Parameter{
				{Key: "podman_registry_password", Value: "resolved-podman-secret"},
			},
			config: CommittedConfig{
				BundleData: BundleData{
					PodmanContainers: &PodmanContainerBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "$(podman_registry_password)"},
						},
					},
				},
			},
			expected: []string{"resolved-podman-secret"},
		},
		{
			name: "secrets from all bundles sorted by length descending",
			storeSecrets: []Parameter{
				{Key: "docker_registry_password", Value: "docker-secret"},
			},
			config: CommittedConfig{
				BundleData: BundleData{
					Parameters: &ParametersBundle{
						Secrets: []Parameter{
							{Key: "db_password", Value: "parameters-registry-secret"},
						},
					},
					DockerCompose: &DockerComposeBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "docker-compose-secret"},
						},
					},
					DockerContainers: &DockerContainersBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "$(docker_registry_password)"},
						},
					},
					PodmanContainers: &PodmanContainerBundle{
						RegistryAuths: []RegistryAuth{
							{Server: "gcr.io", Username: "user", Password: "podman"},
						},
					},
				},
			},
			expected: []string{
				"parameters-registry-secret",
				"docker-compose-secret",
				"docker-secret",
				"podman",
			},
		},
		{
			name: "sorted by length descending to avoid false-prefix-matching",
			config: CommittedConfig{
				BundleData: BundleData{
					Parameters: &ParametersBundle{
						Secrets: []Parameter{
							{Key: "short", Value: "password"},
							{Key: "long", Value: "password123"},
						},
					},
				},
			},
			expected: []string{"password123", "password"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paramsBundle := &ParametersBundle{Secrets: tt.storeSecrets}
			ctx := paramsBundle.Context(context.Background(), new(mockURLSigner))

			assert.Equal(t, tt.config.SecretsList(ctx), tt.expected)
		})
	}
}
