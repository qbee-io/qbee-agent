package configuration

const (
	BundleSettings             = "settings"
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
)

type Config struct {
	Config CommittedConfig `json:"config"`
}

type CommittedConfig struct {
	CommitID   string     `json:"commit_id"`
	Bundles    []string   `json:"bundles"`
	BundleData BundleData `json:"bundle_data"`
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
func (cc *CommittedConfig) selectBundleByName(bundleName string) Bundle {
	switch bundleName {
	case BundleSettings:
		return cc.BundleData.Settings
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
	default:
		return nil
	}
}

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

	// Software
	SoftwareManagement *SoftwareManagementBundle `json:"software_management,omitempty"`
	DockerContainers   *DockerContainersBundle   `json:"docker_containers,omitempty"`

	// Security
	Password *PasswordBundle `json:"password,omitempty"`
	Firewall *FirewallBundle `json:"firewall,omitempty"`
}
