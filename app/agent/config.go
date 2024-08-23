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

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Default connectivity settings
const (
	DefaultDeviceHubServer = "device.app.qbee.io"
	DefaultDeviceHubPort   = "443"
)

const (
	configFileName = "qbee-agent.json"
	configFileMode = 0600
)

// Config defines the configuration of the agent.
type Config struct {

	// BootstrapKey is the bootstrap key used to bootstrap the device.
	BootstrapKey string `json:"bootstrap_key,omitempty"`

	// Directory where the configuration files of the agent are located.
	Directory string `json:"-"`

	// StateDirectory where the agent's state is persisted.
	StateDirectory string `json:"-"`

	// Device Hub API endpoint
	DeviceHubServer string `json:"server"`
	DeviceHubPort   string `json:"port"`

	// HTTP Proxy configuration
	ProxyServer   string `json:"http_proxy_server,omitempty"`
	ProxyPort     string `json:"http_proxy_port,omitempty"`
	ProxyUser     string `json:"http_proxy_user,omitempty"`
	ProxyPassword string `json:"http_proxy_pass,omitempty"`

	// TPM Configuration
	TPMDevice string `json:"tpm_device,omitempty"`

	// DeviceName is the name of the device - only to be used during bootstrap
	DeviceName string `json:"device_name,omitempty"`

	// DisableRemoteAccess disables remote access.
	DisableRemoteAccess bool `json:"disable_remote_access,omitempty"`

	// CACert is the path to the CA certificate.
	CACert string `json:"ca_cert,omitempty"`
}

// LoadConfig loads config from a provided config file path.
func LoadConfig(configDir, stateDir string) (*Config, error) {
	configFilePath := filepath.Join(configDir, configFileName)

	configBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("error loading config from file %s: %w", configFilePath, err)
	}

	config := new(Config)

	if err = json.Unmarshal(configBytes, config); err != nil {
		return nil, fmt.Errorf("error parsing config file %s: %w", configFilePath, err)
	}

	config.Directory = configDir

	if config.StateDirectory, err = filepath.Abs(stateDir); err != nil {
		return nil, fmt.Errorf("cannot determine state directory path: %w", err)
	}

	// Set default device hub port and server if not set
	if config.DeviceHubServer == "" {
		config.DeviceHubServer = DefaultDeviceHubServer
	}

	if config.DeviceHubPort == "" {
		config.DeviceHubPort = DefaultDeviceHubPort
	}

	return config, nil
}

func (agent *Agent) saveConfig() error {

	if agent.cfg.BootstrapKey != "" {
		agent.cfg.BootstrapKey = ""
	}

	if agent.cfg.DeviceName != "" {
		agent.cfg.DeviceName = ""
	}

	config, err := json.Marshal(agent.cfg)
	if err != nil {
		return fmt.Errorf("error marshaling configuration file: %w", err)
	}

	configPath := filepath.Join(agent.cfg.Directory, configFileName)

	if err = os.WriteFile(configPath, config, configFileMode); err != nil {
		return fmt.Errorf("error writing config file %s: %w", configPath, err)
	}

	return nil
}
