package cmd

import (
	"context"
	"fmt"

	"github.com/qbee-io/qbee-agent/app/agent"
)

const (
	boostrapKeyOption                = "bootstrap-key"
	bootstrapDisableAutoUpdateOption = "no-auto-update"
	bootstrapDeviceHubHostOption     = "device-hub-host"
	bootstrapDeviceHubPortOption     = "device-hub-port"
	bootstrapTPMDeviceOption         = "tpm-device"
	bootstrapProxyHostOption         = "proxy-host"
	bootstrapProxyPortOption         = "proxy-port"
	bootstrapProxyUserOption         = "proxy-user"
	bootstrapProxyPasswordOption     = "proxy-password"
)

var bootstrapCommand = Command{
	Description: "Bootstrap device.",
	Options: []Option{
		{
			Name:     boostrapKeyOption,
			Short:    "k",
			Help:     "Set the bootstrap key found in the user profile.",
			Required: true,
		},
		{
			Name: bootstrapDisableAutoUpdateOption,
			Flag: "true",
			Help: "Disable auto-update.",
		},
		{
			Name:    bootstrapDeviceHubHostOption,
			Help:    "Device Hub API host.",
			Hidden:  true,
			Default: agent.DefaultDeviceHubServer,
		},
		{
			Name:    bootstrapDeviceHubPortOption,
			Help:    "Device Hub API port.",
			Hidden:  true,
			Default: agent.DefaultDeviceHubPort,
		},
		{
			Name:  bootstrapTPMDeviceOption,
			Short: "t",
			Help:  "TPM device to use (e.g. /dev/tpm0).",
		},
		{
			Name: bootstrapProxyHostOption,
			Help: "HTTP proxy host to use.",
		},
		{
			Name: bootstrapProxyPortOption,
			Help: "HTTP proxy port to use.",
		},
		{
			Name: bootstrapProxyUserOption,
			Help: "HTTP proxy username.",
		},
		{
			Name: bootstrapProxyPasswordOption,
			Help: "HTTP proxy password.",
		},
	},

	Target: func(opts Options) error {
		cfg := &agent.Config{
			Directory:       opts[mainConfigDirOption],
			StateDirectory:  opts[mainStateDirOption],
			AutoUpdate:      opts[bootstrapDisableAutoUpdateOption] != "true",
			DeviceHubServer: opts[bootstrapDeviceHubHostOption],
			DeviceHubPort:   opts[bootstrapDeviceHubPortOption],
			TPMDevice:       opts[bootstrapTPMDeviceOption],
			ProxyServer:     opts[bootstrapProxyHostOption],
			ProxyPort:       opts[bootstrapProxyPortOption],
			ProxyUser:       opts[bootstrapProxyUserOption],
			ProxyPassword:   opts[bootstrapProxyPasswordOption],
		}

		bootstrapKey, ok := opts[boostrapKeyOption]
		if !ok {
			return fmt.Errorf("bootstrap key (-k) is required")
		}

		ctx := context.Background()

		if err := agent.Bootstrap(ctx, cfg, bootstrapKey); err != nil {
			return fmt.Errorf("bootstrap error: %w", err)
		}

		return nil
	},
}
