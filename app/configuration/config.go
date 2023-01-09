package configuration

type Config struct {
	Config CommittedConfig `json:"config"`
}

type CommittedConfig struct {
	CommitID   string     `json:"commit_id"`
	Bundles    []string   `json:"bundles"`
	BundleData BundleData `json:"bundle_data"`
}

// selectBundleByName returns Bundle by name from the CommittedConfig.
// If unsupported bundle is provided, nil will be returned.
func (cc *CommittedConfig) selectBundleByName(bundleName string) Bundle {
	switch bundleName {
	case "settings":
		return cc.BundleData.Settings
	case "file_distribution":
		return cc.BundleData.FileDistribution
	case "users":
		return cc.BundleData.Users
	default:
		return nil
	}
}

type BundleData struct {
	// Settings
	Settings SettingsBundle `json:"settings"`

	// System
	Users                UsersBundle            `json:"users"`
	SSHKeys              SSHKeys                `json:"sshkeys"`
	PackageManagement    PackageManagement      `json:"package_management"`
	FileDistribution     FileDistributionBundle `json:"file_distribution"`
	ConnectivityWatchdog ConnectivityWatchdog   `json:"connectivity_watchdog"`
	ProcWatch            ProcWatch              `json:"proc_watch"`
	NTP                  NTP                    `json:"ntp"`

	// Software
	SoftwareManagement Management       `json:"software_management"`
	DockerContainers   DockerContainers `json:"docker_containers"`

	// Security
	Password Password `json:"password"`
	Firewall Firewall `json:"firewall"`
}
