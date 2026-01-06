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

import (
	"context"
	"fmt"

	"go.qbee.io/agent/app/software"
)

// PackageManagementBundle controls system packages.
//
// Example payload:
//
//	{
//	 "pre_condition": "test command",
//	 "items": [
//	   {
//	     "name": "httpd2",
//	     "version": "1.2.3"
//	   }
//	 ],
//	 "reboot_mode": "always",
//	 "full_upgrade": false
//	}
type PackageManagementBundle struct {
	Metadata

	PreCondition string     `json:"pre_condition"`
	RebootMode   RebootMode `json:"reboot_mode"`
	FullUpgrade  bool       `json:"full_upgrade"`
	Packages     []Package  `json:"items"`
}

// RebootMode defines whether system should be rebooted after package maintenance or not.
type RebootMode string

// Supported reboot modes.
const (
	// RebootNever means that system will never be rebooted after package maintenance.
	RebootNever RebootMode = "never"

	// RebootAlways means that system will always be rebooted after package maintenance.
	RebootAlways RebootMode = "always"
)

// Package defines a package to be maintained in the system.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Execute package management configuration bundle.
func (p PackageManagementBundle) Execute(ctx context.Context, service *Service) error {
	if !CheckPreCondition(ctx, p.PreCondition) {
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
		updated, err = p.fullUpgrade(ctx, pkgManager, service.elevationCommand)
	} else {
		updated, err = p.partialUpgrade(ctx, pkgManager, service.elevationCommand)
	}

	if updated && p.RebootMode == RebootAlways {
		service.RebootAfterRun(ctx)
	}

	return err
}

// fullUpgrade performs full system upgrade and reports the results.
func (p PackageManagementBundle) fullUpgrade(ctx context.Context, pkgManager software.PackageManager, elevationCmd []string) (bool, error) {
	updated, output, err := pkgManager.UpgradeAll(ctx, elevationCmd)
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
func (p PackageManagementBundle) partialUpgrade(ctx context.Context, pkgManager software.PackageManager, elevationCmd []string) (bool, error) {
	if len(p.Packages) == 0 {
		return false, nil
	}

	installedPackages, err := pkgManager.ListPackages(ctx, elevationCmd)
	if err != nil {
		return false, err
	}

	installedPackagesMap := make(map[string]*software.Package)
	for i := range installedPackages {
		installedPackagesMap[installedPackages[i].Name] = &installedPackages[i]
	}

	packagesInstalled := false

	for _, pkg := range p.Packages {
		pkg.Name = resolveParameters(ctx, pkg.Name)
		pkg.Version = resolveParameters(ctx, pkg.Version)

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

		output, err := pkgManager.Install(ctx, pkg.Name, pkg.Version, elevationCmd)
		if err != nil {
			ReportError(ctx, err, "Unable to install package '%s'", pkg.Name)
			return false, err
		}

		ReportInfo(ctx, output, "Package '%s' successfully installed.", pkg.Name)
		packagesInstalled = true
	}

	return packagesInstalled, nil
}
