package configuration

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

	// Metrics collection enabled.
	Metrics bool `json:"metrics"`

	// Reports collection enabled.
	Reports bool `json:"reports"`

	// RemoteConsole access enabled.
	RemoteConsole bool `json:"remoteconsole"`

	// SoftwareInventory collection enabled.
	SoftwareInventory bool `json:"software_inventory"`

	// ProcessInventory collection enabled.
	ProcessInventory bool `json:"process_inventory"`

	// AgentInterval defines how often agent reports back to the device hub (in minutes).
	AgentInterval int `json:"agentinterval"`
}
