package inventory

import (
	"context"
	"fmt"

	software2 "github.com/qbee-io/qbee-agent/app/software"
)

const TypeSoftware Type = "software"

type Software struct {
	// PackageManager - type of package manager generating the report
	PackageManager software2.PackageManagerType `json:"pkg_manager"`

	// Items - list of installed software
	Items []software2.Package `json:"items"`
}

// CollectSoftwareInventory returns populated Software inventory based on current system status.
func CollectSoftwareInventory(ctx context.Context) (*Software, error) {
	pkgManager := software2.DefaultPackageManager
	if pkgManager == nil {
		return nil, fmt.Errorf("no supported package manager found")
	}

	pkgList, err := pkgManager.ListPackages(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing packages: %w", err)
	}

	softwareInventory := &Software{
		PackageManager: pkgManager.Type(),
		Items:          pkgList,
	}

	return softwareInventory, nil
}
