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

// BundleCommitID return bundle commit ID for the current settings.
func (s SettingsBundle) BundleCommitID(committedConfig *CommittedConfig) string {
	return committedConfig.BundleData.Settings.CommitID
}

// Execute settings config on the system.
func (s SettingsBundle) Execute(_ context.Context, service *Service, configData *CommittedConfig) error {
	settings := configData.BundleData.Settings

	service.reportingEnabled = settings.EnableReports
	service.metricsEnabled = settings.EnableMetrics
	service.remoteConsoleEnabled = settings.EnableRemoteConsole
	service.softwareInventoryEnabled = settings.EnableSoftwareInventory
	service.processInventoryEnabled = settings.EnableProcessInventory
	service.runInterval = settings.RunInterval

	return nil
}
