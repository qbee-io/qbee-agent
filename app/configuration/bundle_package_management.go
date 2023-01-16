package configuration

import (
	"context"
	"fmt"

	"github.com/qbee-io/qbee-agent/app/inventory/software"
	"github.com/qbee-io/qbee-agent/app/utils"
)

// PackageManagementBundle controls system packages.
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
type PackageManagementBundle struct {
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

// Execute package management configuration bundle.
func (p PackageManagementBundle) Execute(ctx context.Context, service *Service) error {
	if !p.checkPreCondition(ctx) {
		return nil
	}

	pkgManager := software.DefaultPackageManager

	if pkgManager == nil {
		ReportError(ctx, nil, "Unsupported package manager.")
		return fmt.Errorf("unuspported package manager")
	}

	if busy, err := pkgManager.Busy(); err != nil {
		ReportError(ctx, err, "Package manager error.")
		return err
	} else if busy {
		return nil
	}

	var err error
	var updated bool

	if p.FullUpgrade {
		updated, err = p.fullUpgrade(ctx, pkgManager)
	} else {
		updated, err = p.partialUpgrade(ctx, pkgManager)
	}

	if updated && p.RebootMode == RebootAlways {
		service.RebootAfterRun(ctx)
	}

	return err
}

// checkPreCondition returns true if pre-condition succeeds or is not defined.
func (p PackageManagementBundle) checkPreCondition(ctx context.Context) bool {
	if p.PreCondition == "" {
		return true
	}

	// return with no error when pre-condition fails
	if _, err := utils.RunCommand(ctx, []string{getShell(), "-c", p.PreCondition}); err != nil {
		return false
	}

	return true
}

// fullUpgrade performs full system upgrade and reports the results.
func (p PackageManagementBundle) fullUpgrade(ctx context.Context, pkgManager software.PackageManager) (bool, error) {
	updated, output, err := pkgManager.UpgradeAll(ctx)
	if err != nil {
		ReportError(ctx, err, "Full upgrade failed.")
		return false, err
	}

	if updated == 0 {
		return false, nil
	}

	ReportInfo(ctx, output, "Full upgrade was successful - %d packages updated.", updated)

	return true, nil
}

// partialUpgrade performs update only of the packages specified in the bundle.
func (p PackageManagementBundle) partialUpgrade(ctx context.Context, pkgManager software.PackageManager) (bool, error) {
	if len(p.Packages) == 0 {
		return false, nil
	}

	installedPackages, err := pkgManager.ListPackages(ctx)
	if err != nil {
		return false, err
	}

	installedPackagesMap := make(map[string]*software.Package)
	for i := range installedPackages {
		installedPackagesMap[installedPackages[i].Name] = &installedPackages[i]
	}

	packagesInstalled := false

	for _, pkg := range p.Packages {
		if pkg.Version == "latest" {
			pkg.Version = ""
		}

		if installedPackage, isInstalled := installedPackagesMap[pkg.Name]; isInstalled {
			alreadyRightVersion := installedPackage.Version == pkg.Version
			alreadyLatestVersion := pkg.Version == "" && installedPackage.Update == ""

			if alreadyRightVersion || alreadyLatestVersion {
				continue
			}
		}

		output, err := pkgManager.Install(ctx, pkg.Name, pkg.Version)
		if err != nil {
			ReportError(ctx, err, "Unable to install package '%s'", pkg.Name)
			return false, err
		}

		ReportInfo(ctx, output, "Package '%s' successfully installed.", pkg.Name)
		packagesInstalled = true
	}

	return packagesInstalled, nil
}
