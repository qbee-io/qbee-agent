package software

// PackageManagerType defines package manager type.
type PackageManagerType string

// PackageManager defines package manager interface.
type PackageManager interface {
	// IsSupported returns true if package manager is supported by the host system.
	IsSupported() bool

	// ListPackages returns a list of packages with available updates.
	ListPackages() ([]Package, error)
}

// PackageManagers provides a map of all package managers supported by the agent.
var PackageManagers = map[PackageManagerType]PackageManager{
	DebPackageManagerType: new(DebPackageManager),
}
