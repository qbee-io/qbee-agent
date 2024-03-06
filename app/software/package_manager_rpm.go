// Copyright 2024 qbee.io
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

package software

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"go.qbee.io/agent/app/utils"
	"go.qbee.io/agent/app/utils/cache"
)

const PackageManagerTypeRpm PackageManagerType = "rpm"
const rpmFileSuffix string = ".rpm"

const (
	rpmPath = "rpm"
	yumPath = "yum"
)

// RpmPackageManager implements PackageManager interface for RPM based systems (Fedora, CentOS etc.)
type RpmPackageManager struct {
	lock sync.Mutex
}

// Type returns type of the package manager.
func (rpm *RpmPackageManager) Type() PackageManagerType {
	return PackageManagerTypeRpm
}

// FileSuffix returns the file suffix for the package manager.
func (rpm *RpmPackageManager) FileSuffix() string {
	return rpmFileSuffix
}

// IsSupported returns true if package manager is supported by the host system.
func (rpm *RpmPackageManager) IsSupported() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// check if both apt-get and dpkg exist and are executable
	for _, filePath := range []string{rpmPath, yumPath} {
		_, err := exec.LookPath(filePath)
		if err != nil {
			return false
		}
	}

	return true
}

// Busy returns true if package manager is currently busy.
const yumPidPath = "/var/run/yum.pid"

func (rpm *RpmPackageManager) Busy() (bool, error) {
	// /var/run/yum.pid

	rpm.lock.Lock()
	defer rpm.lock.Unlock()

	if _, err := os.Stat(yumPidPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return true, err
	}

	return true, fmt.Errorf("%s exists, yum is already running", yumPidPath)
}

// ListPackages returns a list of packages with available updates.
func (rpm *RpmPackageManager) ListPackages(ctx context.Context) ([]Package, error) {

	rpm.lock.Lock()
	defer rpm.lock.Unlock()

	if cachedPackages, ok := cache.Get(packagesCacheKey); ok {
		return cachedPackages.([]Package), nil
	}

	installedPackages, err := rpm.listInstalledPackages(ctx)
	if err != nil {
		return nil, err
	}

	var availableUpdates map[string]string
	availableUpdates, err = rpm.listAvailableUpdates(ctx)
	if err != nil {
		return nil, err
	}

	for i, pkg := range installedPackages {
		installedPackages[i].Update = availableUpdates[pkg.ID()]
	}
	cache.Set(packagesCacheKey, installedPackages, pkgCacheTTL)

	return installedPackages, nil
}

// parseUpdateAvailableLine parses a line from yum check-updates output.
func (rpm *RpmPackageManager) parseUpdateAvailableLine(line string) *Package {
	parts := strings.Fields(line)
	if len(parts) != 3 {
		return nil
	}

	if !strings.Contains(parts[0], ".") {
		return nil
	}

	nameArch := strings.Split(parts[0], ".")
	if len(nameArch) != 2 {
		return nil
	}

	return &Package{
		Name:         nameArch[0],
		Update:       parts[1],
		Architecture: nameArch[1],
	}
}

// listAvailableUpdates returns a list of available updates.
func (rpm *RpmPackageManager) listAvailableUpdates(ctx context.Context) (map[string]string, error) {
	cmd := []string{
		yumPath,
		"--quiet",
		"--assumeyes",
		"check-update",
	}

	updates := make(map[string]string)

	// yum check-updates returns 100 when there are updates available
	cmdProc := utils.NewCommand(ctx, cmd)

	output, err := cmdProc.CombinedOutput()
	// no error, no updates
	if err == nil {
		return nil, nil
	}

	if err != nil {
		exitError := new(exec.ExitError)
		if errors.As(err, &exitError) {
			if exitError.ExitCode() != 100 {
				return nil, fmt.Errorf("error running command %v: %w\n%s", cmd, err, exitError.Stderr)
			}
		}
	}

	err = utils.ForLines(bytes.NewBuffer(output), func(line string) error {
		pkg := rpm.parseUpdateAvailableLine(strings.TrimSpace(line))
		if pkg != nil {
			updates[pkg.ID()] = pkg.Update
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updates, nil
}

// listInstalledPackages returns a list of installed packages.
func (rpm *RpmPackageManager) listInstalledPackages(ctx context.Context) ([]Package, error) {
	cmd := []string{
		rpmPath,
		`-qa`,
		`--queryformat`,
		`%{name}||%{version}-%{release}||%{arch}\n`,
	}

	installedPackages := make([]Package, 0)

	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		// package||1.1.1-1||x86_64
		parts := strings.Split(line, "||")
		if len(parts) != 3 {
			return nil
		}

		pkg := Package{
			Name:         parts[0],
			Version:      parts[1],
			Architecture: parts[2],
		}

		installedPackages = append(installedPackages, pkg)
		return nil
	})

	if err != nil {
		return nil, err

	}

	return installedPackages, nil
}

// UpgradeAll performs upgrade of all packages.
func (rpm *RpmPackageManager) UpgradeAll(ctx context.Context) (int, []byte, error) {
	// check for updates
	inventory, err := rpm.ListPackages(ctx)
	if err != nil {
		return 0, nil, err
	}

	rpm.lock.Lock()
	defer rpm.lock.Unlock()

	updatesAvailable := 0

	for _, pkg := range inventory {
		if pkg.Update != "" {
			updatesAvailable++
		}
	}

	if updatesAvailable == 0 {
		return 0, nil, nil
	}

	upgradeCommand := []string{
		yumPath,
		"--assumeyes",
		"--quiet",
		"update",
	}

	var output []byte
	if output, err = utils.RunCommand(ctx, upgradeCommand); err != nil {
		return 0, output, err
	}

	cache.Delete(packagesCacheKey)

	return updatesAvailable, output, err
}

// Install ensures a package with provided version number is installed in the system.
func (rpm *RpmPackageManager) Install(ctx context.Context, pkgName, version string) ([]byte, error) {
	rpm.lock.Lock()
	defer rpm.lock.Unlock()

	defer cache.Delete(pkgCacheKeyPrefix)

	if version != "" {
		pkgName = fmt.Sprintf("%s-%s", pkgName, version)
	}

	installCommand := []string{
		yumPath,
		"--assumeyes",
		"--quiet",
		"install",
		pkgName,
	}

	return utils.RunCommand(ctx, installCommand)
}

// InstallLocal package.
func (rpm *RpmPackageManager) InstallLocal(ctx context.Context, pkgFilePath string) ([]byte, error) {
	rpm.lock.Lock()
	defer rpm.lock.Unlock()

	defer cache.Delete(packagesCacheKey)

	installCmd := []string{
		yumPath,
		"--assumeyes",
		"--quiet",
		"install",
		pkgFilePath,
	}

	return utils.RunCommand(ctx, installCmd)
}

// PackageArchitecture returns the architecture of the package manager
func (rpm *RpmPackageManager) PackageArchitecture() (string, error) {
	if cachedArch, ok := cache.Get(pkgArchCacheKey); ok {
		return cachedArch.(string), nil
	}

	cmd := []string{rpmPath, "--eval", "%{_arch}"}

	output, err := utils.RunCommand(context.Background(), cmd)
	if err != nil {
		return "", err
	}

	cache.Set(pkgArchCacheKey, strings.TrimSpace(string(output)), pkgCacheTTL)

	return strings.TrimSpace(string(output)), nil
}

// ParsePackageFile returns a package from a file path.
func (rpm *RpmPackageManager) ParsePackageFile(ctx context.Context, filePath string) (*Package, error) {

	cmd := []string{
		rpmPath,
		"--query",
		"--queryformat",
		`%{name}||%{version}-%{release}||%{arch}`,
		"--package",
		filePath,
	}

	output, err := utils.RunCommand(ctx, cmd)

	if err != nil {
		return nil, err
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "||")

	if len(parts) != 3 {
		return nil, fmt.Errorf("error parsing package file %s: unexpected output", filePath)
	}

	return &Package{
		Name:         parts[0],
		Version:      parts[1],
		Architecture: parts[2],
	}, nil
}
