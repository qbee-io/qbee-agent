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
	"path/filepath"
	"strings"

	"go.qbee.io/agent/app/software"
	"go.qbee.io/agent/app/utils"
)

// SoftwareManagementBundle controls software in the system.
//
// Example payload:
//
//	{
//	 "items": [
//	   {
//	     "package": "pkg1",
//	     "service_name": "serviceName",
//	     "config_files": [
//	       {
//	         "config_template": "configFileTemplate",
//	         "config_location": "configFileLocation"
//	       }
//	     ],
//	     "parameters": [
//	       {
//	         "key": "configKey",
//	         "value": "configValue"
//	       }
//	     ]
//	   }
//	 ]
//	}
type SoftwareManagementBundle struct {
	Metadata

	Items []Software `json:"items"`
}

// Execute software management bundle on the system.
func (s SoftwareManagementBundle) Execute(ctx context.Context, srv *Service) error {
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

	for _, item := range s.Items {
		if err := item.Execute(ctx, srv, pkgManager); err != nil {
			return err
		}
	}

	return nil
}

// ConfigFile definition.
type ConfigFile struct {
	// ConfigTemplate defines a source template file from file manager.
	ConfigTemplate string `json:"config_template"`

	// ConfigLocation defines an absolute path in the system where file will be created.
	ConfigLocation string `json:"config_location"`
}

// Software defines software to be maintained in the system.
type Software struct {
	// Package defines a package name to install.
	Package string `json:"package"`

	// ServiceName defines an optional service name (if empty, Package is used).
	ServiceName string `json:"service_name"`

	// PreCondition defines an optional command which needs to return 0 in order for the Software to be installed.
	PreCondition string `json:"pre_condition,omitempty"`

	// ConfigFiles to be created for the software.
	ConfigFiles []ConfigFile `json:"config_files"`

	// Parameters for the ConfigFiles templating.
	Parameters []TemplateParameter `json:"parameters"`
}

func (s Software) serviceName(ctx context.Context, srv *Service) string {
	if s.ServiceName != "" {
		return s.ServiceName
	}

	if strings.HasSuffix(s.Package, software.DefaultPackageManager.FileSuffix()) {
		// since this is executed after s.installFromFile, we can depend on the package being downloaded
		// in the cache and parse correctly, so we are not too concerned about proper error handling here.
		pkgFileCachePath := filepath.Join(srv.cacheDirectory, SoftwareCacheDirectory, s.Package)

		pkgInfo, err := software.DefaultPackageManager.ParsePackageFile(ctx, pkgFileCachePath)
		if err != nil {
			return ""
		}

		return pkgInfo.Name
	}

	return s.Package
}

// Execute a Software configuration on the system.
func (s Software) Execute(ctx context.Context, srv *Service, pkgManager software.PackageManager) error {
	if !CheckPreCondition(ctx, s.PreCondition) {
		return nil
	}

	s.Package = resolveParameters(ctx, s.Package)
	s.ServiceName = resolveParameters(ctx, s.ServiceName)

	var err error
	var shouldRestart bool

	// install package
	if strings.HasSuffix(s.Package, software.DefaultPackageManager.FileSuffix()) {
		shouldRestart, err = s.installFromFile(ctx, srv, pkgManager)
	} else {
		shouldRestart, err = s.installFromRepository(ctx, pkgManager)
	}
	if err != nil {
		return err
	}

	// download config files
	for _, cfgFile := range s.ConfigFiles {
		var created bool

		parameters := templateParametersMap(s.Parameters)
		created, err = srv.downloadTemplateFile(ctx, "", cfgFile.ConfigTemplate, cfgFile.ConfigLocation, parameters)
		if err != nil {
			return err
		}

		if created {
			shouldRestart = true
		}
	}

	// restart service if needed
	if shouldRestart {
		s.restart(ctx, srv)
	}

	return nil
}

// installFromFile installs package from a file.
func (s Software) installFromFile(ctx context.Context, srv *Service, pkgManager software.PackageManager) (bool, error) {
	// download package from the file manager into software cache directory
	var pkgFileCachePath string

	if strings.HasPrefix(s.Package, localFileSchema) {
		pkgFileCachePath = strings.TrimPrefix(s.Package, localFileSchema)
	} else {
		pkgFileCachePath = filepath.Join(srv.cacheDirectory, SoftwareCacheDirectory, s.Package)

		if _, err := srv.downloadFile(ctx, "", s.Package, pkgFileCachePath); err != nil {
			return false, err
		}
	}

	// get package info
	pkgInfo, err := software.DefaultPackageManager.ParsePackageFile(ctx, pkgFileCachePath)
	if err != nil {
		return false, err
	}

	// check non-empty architecture
	if pkgInfo.Architecture == "" {
		ReportError(ctx, nil, "Package %s reports empty architecture", pkgInfo.Name)
		return false, fmt.Errorf("package %s reports empty architecture", pkgInfo.Name)
	}

	// check package architecture
	if err := pkgManager.IsSupportedArchitecture(pkgInfo.Architecture); err != nil {
		ReportError(ctx, err, "Unable to determine supported architecture for package %s", pkgInfo.Name)
		return false, err
	}

	// Check whether package is installed
	if isInstalled, err := s.isPackageInstalled(ctx, pkgInfo, pkgManager); err != nil {
		return false, err
	} else if isInstalled {
		return false, nil
	}

	// install package using the package manager
	var output []byte
	if output, err = pkgManager.InstallLocal(ctx, pkgFileCachePath); err != nil {
		ReportError(ctx, err, "Unable to install '%s'", s.Package)
		return false, err
	}

	// Verify that package was installed
	if isInstalled, err := s.isPackageInstalled(ctx, pkgInfo, pkgManager); err != nil {
		ReportError(ctx, err, "Unable to verify installation of '%s'", s.Package)
		return false, err
	} else if !isInstalled {
		ReportError(ctx, output, "Unable to install '%s'", s.Package)
		return false, fmt.Errorf("unable to install '%s'", s.Package)
	}

	ReportInfo(ctx, output, "Successfully installed '%s'", s.Package)

	return true, nil
}

func (s Software) isPackageInstalled(ctx context.Context, pkgInfo *software.Package, pkgManager software.PackageManager) (bool, error) {
	// check if package is already installed
	installedPackages, err := pkgManager.ListPackages(ctx)
	if err != nil {
		return false, err
	}

	for _, pkg := range installedPackages {
		// continue if name do not match
		if pkg.Name != pkgInfo.Name {
			continue
		}
		// name matches and we do not have version information
		if pkgInfo.Version == "" {
			return true, nil
		}
		// name matches, continue if versions do not match
		if pkg.Version != pkgInfo.Version {
			continue
		}
		// name and version match, return if architecture is the same
		if pkg.Architecture == pkgInfo.Architecture {
			return true, nil
		}
	}
	return false, nil
}

// installFromRepository install package from package repository.
func (s Software) installFromRepository(ctx context.Context, pkgManager software.PackageManager) (bool, error) {
	// Check whether package is installed
	pkgInfo := &software.Package{
		Name: s.Package,
	}
	if isInstalled, err := s.isPackageInstalled(ctx, pkgInfo, pkgManager); err != nil {
		return false, err
	} else if isInstalled {
		return false, nil
	}

	// install package
	var output []byte
	var err error
	if output, err = pkgManager.Install(ctx, s.Package, ""); err != nil {
		ReportError(ctx, err, "Unable to install '%s'", s.Package)
		return false, err
	}

	ReportInfo(ctx, output, "Successfully installed '%s'", s.Package)

	return true, nil
}

// restart restarts the service
func (s Software) restart(ctx context.Context, srv *Service) {
	var err error

	defer func() {
		if err != nil {
			ReportWarning(ctx, err, "Required restart of '%s' cannot be performed", s.Package)
		}
	}()

	serviceName := s.serviceName(ctx, srv)
	if serviceName == "" {
		err = fmt.Errorf("cannot determine service name")
		return
	}

	cmd, err := utils.GenerateServiceCommand(ctx, serviceName, "restart")
	if err != nil {
		return
	}

	if cmd == nil {
		// no restart command was found, we do not have a service
		return
	}

	var output []byte
	if output, err = utils.RunCommand(ctx, cmd); err != nil {
		return
	}

	ReportInfo(ctx, output, "Restarted service '%s'", serviceName)
}
