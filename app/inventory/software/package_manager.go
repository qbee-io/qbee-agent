package software

import "context"

// DefaultPackageManager is set to the supporter package manager for the OS.
// If there are no support package managers available, this will be nil.
var DefaultPackageManager PackageManager

// PackageManagers provides a map of all package managers supported by the agent.
var PackageManagers = map[PackageManagerType]PackageManager{
	PackageManagerTypeDebian: new(DebianPackageManager),
}

func init() {
	// set default package manager
	for _, pkgManager := range PackageManagers {
		if pkgManager.IsSupported() {
			DefaultPackageManager = pkgManager
			return
		}
	}
}

// PackageManagerType defines package manager type.
type PackageManagerType string

// PackageManager defines package manager interface.
type PackageManager interface {
	Type() PackageManagerType

	// IsSupported returns true if package manager is supported by the host system.
	IsSupported() bool

	// Busy returns true if package manager is currently busy.
	Busy() (bool, error)

	// ListPackages returns a list of packages with available updates.
	ListPackages(ctx context.Context) ([]Package, error)

	// UpgradeAll performs upgrade of all packages.
	// On success, return number of packages upgraded, output of the upgrade command and nil error.
	UpgradeAll(ctx context.Context) (int, []byte, error)

	// Install ensures a package with provided version number is installed in the system.
	// If package exists in the system in the right version, return false.
	// If package was installed as a result of this method call, return true.
	Install(ctx context.Context, pkgName, version string) ([]byte, error)
}
