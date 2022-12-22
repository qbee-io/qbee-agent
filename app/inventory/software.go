package inventory

import (
	"fmt"

	"github.com/qbee-io/qbee-agent/app/inventory/software"
)

type Software struct {
	// PackageManager - type of package manager generating the report
	PackageManager software.PackageManagerType `json:"pkg_manager"`

	// Items - list of installed software
	Items []software.Package `json:"items"`
}

// CollectSoftwareInventory returns populated Software inventory based on current system status.
func CollectSoftwareInventory(pkgManagerType software.PackageManagerType) (*Software, error) {
	pkgManager, ok := software.PackageManagers[pkgManagerType]
	if !ok {
		return nil, fmt.Errorf("unknown package manager type: %s", pkgManagerType)
	}

	if !pkgManager.IsSupported() {
		return nil, nil
	}

	pkgList, err := pkgManager.ListPackages()
	if err != nil {
		return nil, fmt.Errorf("error listing %s packages: %w", pkgManagerType, err)
	}

	softwareInventory := &Software{
		PackageManager: pkgManagerType,
		Items:          pkgList,
	}

	return softwareInventory, nil
}
