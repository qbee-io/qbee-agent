package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultDeviceHubServer = "device.app.qbee.io"
	DefaultDeviceHubPort   = "443"
	DefaultVPNServer       = "vpn.app.qbee.io"
)

const (
	ConfigFileName = "qbee-agent.json"
	ConfigFileMode = 0600
)

type Config struct {
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

	// AutoUpdate enables automatic updates of the agent binary.
	AutoUpdate bool `json:"auto_update,omitempty"`

	// VPNServer is the IP address of the VPN server to connect to.
	VPNServer string `json:"vpn_server,omitempty"`
}

// LoadConfig loads config from a provided config file path.
func LoadConfig(configDir, stateDir string) (*Config, error) {
	configFilePath := filepath.Join(configDir, ConfigFileName)

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

	// vpn_server is not a legacy configuration option, if it is not set, use the default
	if config.VPNServer == "" {
		config.VPNServer = DefaultVPNServer
	}

	return config, nil
}

func (agent *Agent) saveConfig() error {
	config, err := json.Marshal(agent.cfg)
	if err != nil {
		return fmt.Errorf("error marshaling configuration file: %w", err)
	}

	configPath := filepath.Join(agent.cfg.Directory, ConfigFileName)

	if err = os.WriteFile(configPath, config, ConfigFileMode); err != nil {
		return fmt.Errorf("error writing config file %s: %w", configPath, err)
	}

	return nil
}
