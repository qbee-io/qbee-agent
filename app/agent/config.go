package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
)

const (
	DefaultDeviceHubServer = "device.app.qbee.io"
	DefaultDeviceHubPort   = "443"
)

const (
	DefaultConfigDir = "/etc/qbee/"
	ConfigFileName   = "qbee-agent.json"
	ConfigFileMode   = 0600
)

type Config struct {
	// Directory where the configuration files of the agent are located.
	Directory string `json:"-"`

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

	// AutoUpdate enables auto-update functionality
	AutoUpdate bool `json:"auto_update,omitempty"`
}

// LoadConfig loads config from a provided config file path.
func LoadConfig(configDir string) (*Config, error) {
	configFilePath := path.Join(configDir, ConfigFileName)

	configBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("error loading config from file %s: %w", configFilePath, err)
	}

	config := new(Config)

	if err = json.Unmarshal(configBytes, config); err != nil {
		return nil, fmt.Errorf("error parsing config file %s: %w", configFilePath, err)
	}

	config.Directory = configDir

	return config, nil
}

func (agent *Agent) saveConfig() error {
	config, err := json.Marshal(agent.cfg)
	if err != nil {
		return fmt.Errorf("error marshaling configuration file: %w", err)
	}

	configPath := path.Join(agent.cfg.Directory, ConfigFileName)

	if err = os.WriteFile(configPath, config, ConfigFileMode); err != nil {
		return fmt.Errorf("error writing config file %s: %w", configPath, err)
	}

	return nil
}
