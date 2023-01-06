package configuration

// PackageManagement controls system packages.
//
// Example payload:
// {
//  "pre_condition": "test command",
//  "items": [
//    {
//      "name": "httpd2",
//      "version": "1.2.3"
//    }
//  ],
//  "reboot_mode": "always",
//  "full_upgrade": false
// }
type PackageManagement struct {
	Metadata

	PreCondition string     `json:"pre_condition"`
	RebootMode   RebootMode `json:"reboot_mode"`
	FullUpgrade  bool       `json:"full_upgrade"`
	Packages     []Package  `json:"items"`
}

// RebootMode defines whether system should be rebooted after package maintanance or not.
type RebootMode string

const (
	RebootNever  RebootMode = "never"
	RebootAlways RebootMode = "always"
)

// Package defines a package to be maintained in the system.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
