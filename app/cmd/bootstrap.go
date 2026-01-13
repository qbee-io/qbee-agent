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
	"go.qbee.io/agent/app/utils"
	"go.qbee.io/agent/app/utils/cmd"
)

const (
	bootstrapKeyOption                 = "bootstrap-key"
	bootstrapDeviceHubHostOption       = "device-hub-host"
	bootstrapDeviceHubPortOption       = "device-hub-port"
	bootstrapTPMDeviceOption           = "tpm-device"
	bootstrapProxyHostOption           = "proxy-host"
	bootstrapProxyPortOption           = "proxy-port"
	bootstrapProxyUserOption           = "proxy-user"
	bootstrapProxyPasswordOption       = "proxy-password"
	bootstrapDeviceNameOption          = "device-name"
	bootstrapDisableRemoteAccessOption = "disable-remote-access"
	bootstrapCACert                    = "ca-cert"
	bootstrapExecUser                  = "exec-user"
	bootstrapUsePrivilegeElevation     = "use-privilege-elevation"
	bootstrapElevationCommand          = "elevation-command"
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
			Name:  bootstrapTPMDeviceOption,
			Short: "t",
			Help:  "[Beta] TPM device to use (e.g. /dev/tpm0).",
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
		{
			Name: bootstrapCACert,
			Help: "Custom CA certificate to use for TLS.",
		},
		{
			Name: bootstrapExecUser,
			Help: "User to run the agent as.",
		},
		{
			Name: bootstrapUsePrivilegeElevation,
			Flag: "true",
			Help: "Use privilege elevation for commands requiring elevated privileges.",
		},
		{
			Name: bootstrapElevationCommand,
			Help: "Command to use for privilege elevation (e.g. sudo). The default \"sudo -n\" " +
				"requires passwordless sudo or appropriate sudoers configuration (e.g. NOPASSWD for the target user).",
			Default: "sudo -n",
		},
	},

	Target: func(opts cmd.Options) error {

		cfg := &agent.Config{
			BootstrapKey:          opts[bootstrapKeyOption],
			Directory:             opts[mainConfigDirOption],
			StateDirectory:        opts[mainStateDirOption],
			DeviceHubServer:       opts[bootstrapDeviceHubHostOption],
			DeviceHubPort:         opts[bootstrapDeviceHubPortOption],
			TPMDevice:             opts[bootstrapTPMDeviceOption],
			ProxyServer:           opts[bootstrapProxyHostOption],
			ProxyPort:             opts[bootstrapProxyPortOption],
			ProxyUser:             opts[bootstrapProxyUserOption],
			ProxyPassword:         opts[bootstrapProxyPasswordOption],
			DeviceName:            opts[bootstrapDeviceNameOption],
			DisableRemoteAccess:   opts[bootstrapDisableRemoteAccessOption] == "true",
			CACert:                opts[bootstrapCACert],
			ExecUser:              opts[bootstrapExecUser],
			UsePrivilegeElevation: opts[bootstrapUsePrivilegeElevation] == "true",
		}

		if cfg.UsePrivilegeElevation {
			elevationCmd, err := utils.ParseCommandLine(opts[bootstrapElevationCommand])
			if err != nil {
				return fmt.Errorf("cannot parse elevation command: %w", err)
			}
			cfg.ElevationCommand = elevationCmd
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
