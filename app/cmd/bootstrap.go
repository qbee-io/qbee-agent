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

package cmd

import (
	"context"
	"fmt"

	"go.qbee.io/agent/app/agent"
	"go.qbee.io/agent/app/utils/cmd"
)

const (
	bootstrapKeyOption                 = "bootstrap-key"
	bootstrapAutoUpdateOption          = "enable-auto-update"
	bootstrapDeviceHubHostOption       = "device-hub-host"
	bootstrapDeviceHubPortOption       = "device-hub-port"
	bootstrapVPNServerOption           = "vpn-server"
	bootstrapTPMDeviceOption           = "tpm-device"
	bootstrapProxyHostOption           = "proxy-host"
	bootstrapProxyPortOption           = "proxy-port"
	bootstrapProxyUserOption           = "proxy-user"
	bootstrapProxyPasswordOption       = "proxy-password"
	bootstrapDeviceNameOption          = "device-name"
	bootstrapDisableRemoteAccessOption = "disable-remote-access"
)

var bootstrapCommand = cmd.Command{
	Description: "Bootstrap device.",
	Options: []cmd.Option{
		{
			Name:     bootstrapKeyOption,
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
			Name: bootstrapDisableRemoteAccessOption,
			Flag: "true",
			Help: "Disable remote access.",
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
			Name: bootstrapDeviceNameOption,
			Help: "Custom device name to use.",
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
			BootstrapKey:        opts[bootstrapKeyOption],
			Directory:           opts[mainConfigDirOption],
			StateDirectory:      opts[mainStateDirOption],
			AutoUpdate:          opts[bootstrapAutoUpdateOption] == "true",
			DeviceHubServer:     opts[bootstrapDeviceHubHostOption],
			DeviceHubPort:       opts[bootstrapDeviceHubPortOption],
			VPNServer:           opts[bootstrapVPNServerOption],
			TPMDevice:           opts[bootstrapTPMDeviceOption],
			ProxyServer:         opts[bootstrapProxyHostOption],
			ProxyPort:           opts[bootstrapProxyPortOption],
			ProxyUser:           opts[bootstrapProxyUserOption],
			ProxyPassword:       opts[bootstrapProxyPasswordOption],
			DeviceName:          opts[bootstrapDeviceNameOption],
			DisableRemoteAccess: opts[bootstrapDisableRemoteAccessOption] == "true",
		}

		if cfg.BootstrapKey == "" {
			return fmt.Errorf("bootstrap key (-k) is required")
		}

		ctx := context.Background()

		if err := agent.Bootstrap(ctx, cfg); err != nil {
			return fmt.Errorf("bootstrap error: %w", err)
		}

		return nil
	},
}
