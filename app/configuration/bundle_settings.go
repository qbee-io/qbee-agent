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
)

// SettingsBundle
//
// Example payload:
// "settings": {
//   "metrics": true,
//   "reports": true,
//   "remoteconsole": true,
//   "software_inventory": true,
//   "process_inventory": true,
//   "agentinterval": 10
// }
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
}

// Execute settings config on the system.
func (s SettingsBundle) Execute(service *Service) {
	service.reportingEnabled = s.EnableReports
	service.metricsEnabled = s.EnableMetrics
	service.remoteConsoleEnabled = s.EnableRemoteConsole
	service.softwareInventoryEnabled = s.EnableSoftwareInventory
	service.processInventoryEnabled = s.EnableProcessInventory

	if service.runInterval != s.RunInterval {
		service.runIntervalChangeNotifier <- time.Duration(s.RunInterval) * time.Minute
	}

	service.runInterval = s.RunInterval
}
