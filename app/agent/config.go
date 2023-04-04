package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/qbee-io/qbee-agent/app/log"
)

const (
	DefaultDeviceHubServer = "device.app.qbee.io"
	DefaultDeviceHubPort   = "443"
)

const (
	ConfigFileName = "qbee-agent.json"
	ConfigFileMode = 0600
)

type Config struct {
	// Directory where the configuration files of the agent are located.
	Directory string `json:"-"`

	// CacheDirectory where the agent's cache is located.
	CacheDirectory string `json:"-"`

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

	// DisableAutoUpdate functionality
	DisableAutoUpdate bool `json:"disable_auto_update,omitempty"`
}

// LoadConfig loads config from a provided config file path.
// If the config file does not exist, it will return a default configuration.
func LoadConfig(configDir, cacheDir string) (*Config, error) {
	config := new(Config)

	var err error

	if config.Directory, err = filepath.Abs(configDir); err != nil {
		return nil, fmt.Errorf("error resolving config directory path: %w", err)
	}

	if config.CacheDirectory, err = filepath.Abs(cacheDir); err != nil {
		return nil, fmt.Errorf("error resolving cache directory path: %w", err)
	}

	configFilePath := filepath.Join(configDir, ConfigFileName)

	var configBytes []byte
	if configBytes, err = os.ReadFile(configFilePath); err != nil {
		if os.IsNotExist(err) {
			log.Warnf("config file %s does not exist, using default configuration", configFilePath)
			return config, nil
		}

		return nil, fmt.Errorf("error loading config from file %s: %w", configFilePath, err)
	}

	if err = json.Unmarshal(configBytes, config); err != nil {
		return nil, fmt.Errorf("error parsing config file %s: %w", configFilePath, err)
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
