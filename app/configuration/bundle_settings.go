package configuration

import (
	"context"
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
func (s SettingsBundle) Execute(_ context.Context, service *Service) error {
	service.reportingEnabled = s.EnableReports
	service.metricsEnabled = s.EnableMetrics
	service.remoteConsoleEnabled = s.EnableRemoteConsole
	service.softwareInventoryEnabled = s.EnableSoftwareInventory
	service.processInventoryEnabled = s.EnableProcessInventory
	service.runInterval = s.RunInterval

	return nil
}
