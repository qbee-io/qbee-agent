package software

import "fmt"

type Package struct {
	// Name - package name
	Name string `json:"name"`

	// Version - package version
	Version string `json:"version"`

	// Architecture - package architecture
	Architecture string `json:"arch,omitempty"`

	// Update - available package version upgrade
	Update string `json:"update,omitempty"`
}

// ID software package identifier.
func (pkg *Package) ID() string {
	if pkg.Architecture == "" {
		return pkg.Name
	}

	return fmt.Sprintf("%s:%s", pkg.Name, pkg.Architecture)
}
