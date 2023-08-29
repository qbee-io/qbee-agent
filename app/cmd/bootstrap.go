package cmd

import (
	"context"
	"fmt"

	"qbee.io/platform/utils/cmd"

	"github.com/qbee-io/qbee-agent/app/agent"
)

const (
	boostrapKeyOption            = "bootstrap-key"
	bootstrapAutoUpdateOption    = "enable-auto-update"
	bootstrapDeviceHubHostOption = "device-hub-host"
	bootstrapDeviceHubPortOption = "device-hub-port"
	bootstrapVPNServerOption     = "vpn-server"
	bootstrapTPMDeviceOption     = "tpm-device"
	bootstrapProxyHostOption     = "proxy-host"
	bootstrapProxyPortOption     = "proxy-port"
	bootstrapProxyUserOption     = "proxy-user"
	bootstrapProxyPasswordOption = "proxy-password"
)

var bootstrapCommand = cmd.Command{
	Description: "Bootstrap device.",
	Options: []cmd.Option{
		{
			Name:     boostrapKeyOption,
			Short:    "k",
			Help:     "Set the bootstrap key found in the user profile.",
			Required: true,
		},
		{
			Name: bootstrapAutoUpdateOption,
			Flag: "true",
			Help: "Enable auto-update.",
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
			Name:    bootstrapVPNServerOption,
			Hidden:  true,
			Help:    "VPN Server IP address.",
			Default: agent.DefaultVPNServer,
		},
		{
			Name:  bootstrapTPMDeviceOption,
			Short: "t",
			// Hiding for now, since TPM protected key can't be used with OpenVPN.
			// Once we implement our own remote access solution, this won't be an issue.
			Hidden: true,
			Help:   "[Experimental] TPM device to use (e.g. /dev/tpm0).",
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

	Target: func(opts cmd.Options) error {
		cfg := &agent.Config{
			Directory:       opts[mainConfigDirOption],
			StateDirectory:  opts[mainStateDirOption],
			AutoUpdate:      opts[bootstrapAutoUpdateOption] == "true",
			DeviceHubServer: opts[bootstrapDeviceHubHostOption],
			DeviceHubPort:   opts[bootstrapDeviceHubPortOption],
			VPNServer:       opts[bootstrapVPNServerOption],
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
