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

	"go.qbee.io/agent/app/software"
	"go.qbee.io/agent/app/utils"
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

	// PrivilegeElevation indicates whether to use privilege elevation for commands requiring elevated privileges.
	PrivilegeElevation bool `json:"privilege_elevation,omitempty"`

	// ElevationCommand is the command to use for privilege elevation.
	ElevationCommand []string `json:"elevation_command,omitempty"`
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

	if !config.PrivilegeElevation {
		return config, nil
	}

	// Set default elevation command if not set to use system sudo
	if len(config.ElevationCommand) == 0 {
		// attempt to auto-resolve sudo path
		sudoPath, err := resolveSudoPath()
		if err != nil {
			return nil, fmt.Errorf("elevation command not set and cannot resolve sudo path: %w", err)
		}
		config.ElevationCommand = []string{sudoPath, "-n"}
	}

	if err := ValidateElevationCommand(config.ElevationCommand); err != nil {
		return nil, err
	}

	// propagate elevation command to software package manager
	if software.DefaultPackageManager != nil {
		software.DefaultPackageManager.WithElevationCommand(config.ElevationCommand)
	}

	return config, nil
}

// validateElevationCommand ensures the elevation command is safe to use.
// It only allows a small, known-safe set of elevation tools or absolute paths.
func ValidateElevationCommand(cmd []string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("elevation command is empty")
	}

	// Otherwise require an absolute path to avoid PATH-based attacks.
	if !filepath.IsAbs(cmd[0]) {
		return fmt.Errorf("elevation command %q must be an absolute path", cmd[0])
	}

	// check that the commmand is executable
	if fileInfo, err := os.Stat(cmd[0]); err != nil {
		return fmt.Errorf("cannot stat elevation command %q: %w", cmd[0], err)
	} else if fileInfo.Mode()&0111 == 0 {
		return fmt.Errorf("elevation command %q is not executable", cmd[0])
	}

	return nil
}

func resolveSudoPath() (string, error) {
	systemSudoPaths := []string{
		"/usr/bin/sudo",
		"/bin/sudo",
	}

	for _, path := range systemSudoPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("sudo command not found in standard locations")
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

	// Write file with sync to ensure data is written to disk
	if err = utils.WriteFileSync(configPath, config, configFileMode); err != nil {
		return fmt.Errorf("error writing config file %s: %w", configPath, err)
	}
	return nil
}
