package configuration

const (
	BundleSettings             = "settings"
	BundleFileDistribution     = "file_distribution"
	BundleUsers                = "users"
	BundleSSHKeys              = "sshkeys"
	BundlePackageManagement    = "package_management"
	BundleConnectivityWatchdog = "connectivity_watchdog"
	BundleProcessWatch         = "proc_watch"
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
	default:
		return nil
	}
}

type BundleData struct {
	// Settings
	Settings SettingsBundle `json:"settings"`

	// System
	Users                UsersBundle                `json:"users"`
	SSHKeys              SSHKeysBundle              `json:"sshkeys"`
	PackageManagement    PackageManagementBundle    `json:"package_management"`
	FileDistribution     FileDistributionBundle     `json:"file_distribution"`
	ConnectivityWatchdog ConnectivityWatchdogBundle `json:"connectivity_watchdog"`
	ProcessWatch         ProcessWatchBundle         `json:"proc_watch"`
	NTP                  NTP                        `json:"ntp"`

	// Software
	SoftwareManagement Management       `json:"software_management"`
	DockerContainers   DockerContainers `json:"docker_containers"`

	// Security
	Password Password `json:"password"`
	Firewall Firewall `json:"firewall"`
}
