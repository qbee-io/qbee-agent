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

// Supported configuration bundles.
const (
	BundleSettings             = "settings"
	BundleParameters           = "parameters"
	BundleFileDistribution     = "file_distribution"
	BundleUsers                = "users"
	BundleSSHKeys              = "sshkeys"
	BundlePackageManagement    = "package_management"
	BundleConnectivityWatchdog = "connectivity_watchdog"
	BundleProcessWatch         = "proc_watch"
	BundleNTP                  = "ntp"
	BundleSoftwareManagement   = "software_management"
	BundleFirewall             = "firewall"
	BundlePassword             = "password"
	BundleDockerContainers     = "docker_containers"
	BundleRauc                 = "rauc"
)

// CommittedConfig contains the configuration that is committed.
type CommittedConfig struct {
	// CommitID represents commit ID of the most recent commit affecting the device.
	CommitID string `json:"commit_id"`

	// Bundles contains a list of strings representing configuration bundles
	Bundles []string `json:"bundles"`

	// BundleData contain configuration data for bundles in the Bundles list
	BundleData BundleData `json:"bundle_data"`

	// EdgeURL is the URL of the edge server the agent should connect to enable remote access.
	EdgeURL string `json:"edge_url"`
}

// HasBundle returns true if bundleName is set in the Bundles list.
func (cc *CommittedConfig) HasBundle(bundleName string) bool {
	for _, bundle := range cc.Bundles {
		if bundle == bundleName {
			return true
		}
	}

	return false
}

// selectBundleByName returns Bundle by name from the CommittedConfig.
// If unsupported bundle is provided, nil will be returned.
// Note: settings is not supported here, since it's going through its dedicated flow.
func (cc *CommittedConfig) selectBundleByName(bundleName string) Bundle {
	switch bundleName {
	case BundleFileDistribution:
		return cc.BundleData.FileDistribution
	case BundleUsers:
		return cc.BundleData.Users
	case BundleSSHKeys:
		return cc.BundleData.SSHKeys
	case BundlePackageManagement:
		return cc.BundleData.PackageManagement
	case BundleConnectivityWatchdog:
		return cc.BundleData.ConnectivityWatchdog
	case BundleProcessWatch:
		return cc.BundleData.ProcessWatch
	case BundleNTP:
		return cc.BundleData.NTP
	case BundleSoftwareManagement:
		return cc.BundleData.SoftwareManagement
	case BundleFirewall:
		return cc.BundleData.Firewall
	case BundlePassword:
		return cc.BundleData.Password
	case BundleDockerContainers:
		return cc.BundleData.DockerContainers
	case BundleRauc:
		return cc.BundleData.Rauc
	default:
		return nil
	}
}

// BundleData contains all configuration bundles.
// Only bundles that are mentioned in the CommittedConfig.Bundles list are populated.
type BundleData struct {
	// Settings
	Settings SettingsBundle `json:"settings"`

	// System
	Users                *UsersBundle                `json:"users,omitempty"`
	SSHKeys              *SSHKeysBundle              `json:"sshkeys,omitempty"`
	PackageManagement    *PackageManagementBundle    `json:"package_management,omitempty"`
	FileDistribution     *FileDistributionBundle     `json:"file_distribution,omitempty"`
	ConnectivityWatchdog *ConnectivityWatchdogBundle `json:"connectivity_watchdog,omitempty"`
	ProcessWatch         *ProcessWatchBundle         `json:"proc_watch,omitempty"`
	NTP                  *NTPBundle                  `json:"ntp,omitempty"`
	Parameters           *ParametersBundle           `json:"parameters,omitempty"`

	// Software
	SoftwareManagement *SoftwareManagementBundle `json:"software_management,omitempty"`
	DockerContainers   *DockerContainersBundle   `json:"docker_containers,omitempty"`

	// Security
	Password *PasswordBundle `json:"password,omitempty"`
	Firewall *FirewallBundle `json:"firewall,omitempty"`

	//Image OTA
	Rauc *RaucBundle `json:"rauc,omitempty"`
}
