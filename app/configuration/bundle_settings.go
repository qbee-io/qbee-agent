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

package configuration

import (
	"time"

	"go.qbee.io/agent/app/software"
)

// SettingsBundle defines settings for the agent.
//
// Example payload:
//
//	"settings": {
//	  "metrics": true,
//	  "reports": true,
//	  "remoteconsole": true,
//	  "software_inventory": true,
//	  "process_inventory": true,
//	  "agentinterval": 10,
//	  "pkg_cache_ttl": 24,
//	  "execution_timeout": 60
//	}
type SettingsBundle struct {
	Metadata

	// EnableMetrics collection enabled.
	EnableMetrics bool `json:"metrics"`

	// EnableReports collection enabled.
	EnableReports bool `json:"reports"`

	// EnableRemoteConsole access enabled.
	EnableRemoteConsole bool `json:"remoteconsole"`

	// EnableSoftwareInventory collection enabled.
	EnableSoftwareInventory bool `json:"software_inventory"`

	// EnableProcessInventory collection enabled.
	EnableProcessInventory bool `json:"process_inventory"`

	// RunInterval defines how often agent reports back to the device hub (in minutes).
	RunInterval int `json:"agentinterval"`

	// PkgCacheTTL defines how long the list of installed packages and available updates is cached (in hours).
	// If not set (or not positive), the default of 24 hours is used.
	PkgCacheTTL int `json:"pkg_cache_ttl,omitempty"`

	// ExecutionTimeout defines the time limit of the configuration execution (in minutes).
	// If not set (or not positive), the default of 60 minutes is used. Values below 5 minutes are raised to 5 minutes,
	// and values above 24 hours (1440 minutes) are lowered to 24 hours.
	// File downloads which are still making progress when the time limit is reached are allowed to finish.
	// Note: stopping or restarting the agent service waits for the configuration execution in progress for up to
	// an hour (TimeoutStopSec of the systemd service). Executions taking longer are cut off then, and the remaining
	// bundles are applied during the next run.
	ExecutionTimeout int `json:"execution_timeout,omitempty"`
}

// Execute settings config on the system.
func (s SettingsBundle) Execute(service *Service) {
	service.reportingEnabled.Store(s.EnableReports)
	service.metricsEnabled = s.EnableMetrics
	service.softwareInventoryEnabled.Store(s.EnableSoftwareInventory)
	service.processInventoryEnabled.Store(s.EnableProcessInventory)

	setPkgCacheTTL(time.Duration(s.PkgCacheTTL) * time.Hour)
	service.setExecutionTimeout(s.ExecutionTimeout)

	// Do not accept run interval smaller than the default agent interval.
	if s.RunInterval < minimumAgentInterval {
		return
	}

	if service.runInterval != s.RunInterval {
		service.runIntervalChangeNotifier <- time.Duration(s.RunInterval) * time.Minute
	}

	service.runInterval = s.RunInterval
}

// setPkgCacheTTL sets the package list cache TTL of the default package manager (if any).
// Non-positive value resets it to the default TTL.
func setPkgCacheTTL(ttl time.Duration) {
	if software.DefaultPackageManager == nil {
		return
	}
	software.DefaultPackageManager.SetCacheTTL(ttl)
}
