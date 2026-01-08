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

package software

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"go.qbee.io/agent/app/utils"
	"go.qbee.io/agent/app/utils/cache"
)

// PackageManagerTypeDebian is the type of the Debian package manager.
const PackageManagerTypeDebian PackageManagerType = "deb"
const debianFileSuffix string = ".deb"

var debianPackagesCacheKey = fmt.Sprintf("%s:%s:packages", pkgCacheKeyPrefix, PackageManagerTypeDebian)
var debianPkgArchCacheKey = fmt.Sprintf("%s:%s:arch", pkgCacheKeyPrefix, PackageManagerTypeDebian)

const (
	aptGetPath = "/usr/bin/apt-get"
	dpkgPath   = "/usr/bin/dpkg"

	dpkgLockPath = "/var/lib/dpkg/lock"
	dpkgLockMode = 0640
)

// DebianPackageManager implements PackageManager interface for Debian-based systems.
type DebianPackageManager struct {
	supportsAllowDowngradesFlag bool
	lock                        sync.Mutex
}

// Type returns type of the package manager.
func (deb *DebianPackageManager) Type() PackageManagerType {
	return PackageManagerTypeDebian
}

// FileSuffix returns the file suffix for the package manager.
func (deb *DebianPackageManager) FileSuffix() string {
	return debianFileSuffix
}

// IsSupported returns true if package manager is supported by the host system.
func (deb *DebianPackageManager) IsSupported() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// check if both apt-get and dpkg exist and are executable
	for _, filePath := range []string{aptGetPath, dpkgPath} {
		_, err := exec.LookPath(filePath)
		if err != nil {
			return false
		}
	}

	return true
}

// Busy returns true if dpkg is currently locked.
func (deb *DebianPackageManager) Busy() (bool, error) {
	deb.lock.Lock()
	defer deb.lock.Unlock()

	// cannot check for lock file, best effort
	if os.Geteuid() != 0 {
		return false, nil
	}

	// check the lock by attempting to acquire one
	file, err := os.OpenFile(dpkgLockPath, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC, dpkgLockMode)
	if err != nil {
		return false, fmt.Errorf("cannot open file %s: %w", dpkgLockPath, err)
	}

	defer func() { _ = file.Close() }()

	flockT := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: io.SeekStart,
		Start:  0,
		Len:    0,
	}

	if err = syscall.FcntlFlock(file.Fd(), syscall.F_SETLK, &flockT); err != nil {
		return true, err
	}

	return false, nil
}

// ListPackages returns a list of packages with available updates.
func (deb *DebianPackageManager) ListPackages(ctx context.Context, elevationCmd []string) ([]Package, error) {
	deb.lock.Lock()
	defer deb.lock.Unlock()

	if cachedPackages, ok := cache.Get(debianPackagesCacheKey); ok {
		return cachedPackages.([]Package), nil
	}

	installedPackages, err := deb.listInstalledPackages(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing installed packages: %w", err)
	}

	// availableUpdates = map[pkgName:arch]updateVersion
	var availableUpdates map[string]string
	if availableUpdates, err = deb.listAvailableUpdates(ctx, elevationCmd); err != nil {
		return nil, fmt.Errorf("error listing available updates: %w", err)
	}

	for i, pkg := range installedPackages {
		installedPackages[i].Update = availableUpdates[pkg.ID()]
	}

	cache.Set(debianPackagesCacheKey, installedPackages, pkgCacheTTL)

	return installedPackages, nil
}

// listInstalledPackages returns currently installed debian packages.
func (deb *DebianPackageManager) listInstalledPackages(ctx context.Context) ([]Package, error) {
	cmd := []string{dpkgPath, "-l"}

	installedPackages := make([]Package, 0)

	// only process lines matching the following format:
	// ii  libsystemd0:amd64           232-25+deb9u13     amd64              systemd utility library
	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		fields := strings.Fields(line)

		if fields[0] != "ii" || len(fields) < 4 {
			return nil
		}

		pkg := Package{
			Name:         strings.SplitN(fields[1], ":", 2)[0],
			Version:      fields[2],
			Architecture: fields[3],
		}

		if pkg.Name == "apt" && utils.IsNewerVersion(pkg.Version, "1.1") {
			deb.supportsAllowDowngradesFlag = true
		}

		installedPackages = append(installedPackages, pkg)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return installedPackages, nil
}

// listAvailableUpdates returns a map of pkgName:arch -> availableUpdateVersion for packages with available updates.
func (deb *DebianPackageManager) listAvailableUpdates(ctx context.Context, elevationCmd []string) (map[string]string, error) {
	updateCmd := []string{aptGetPath, "update"}

	if _, err := utils.RunPrivilegedCommand(ctx, elevationCmd, updateCmd); err != nil {
		return nil, err
	}

	updates := make(map[string]string)
	cmd := []string{aptGetPath, "--just-print", "--with-new-pkgs", "upgrade"}

	// only process lines matching the following format:
	// Inst libsystemd0 [232-25+deb9u13] (232-25+deb9u14 Debian-Security:9/oldoldstable [amd64])
	output, err := utils.RunPrivilegedCommand(ctx, elevationCmd, cmd)
	if err != nil {
		return nil, err
	}

	err = utils.ForLines(bytes.NewBuffer(output), func(line string) error {
		pkg := deb.parseUpdateAvailableLine(line)
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

var debPkgUpdateRE = regexp.MustCompile(`^Inst (\S+) (?:\[(.+)] )?\((\S+) .* \[(.*)]\)`)

// parseUpdateAvailableLine parses a line from `apt-get --just-print upgrade` output into a Package.
// If line doesn't match the expected format, nil is returned.
// Supported format:
// Inst libsystemd0 [232-25+deb9u13] (232-25+deb9u14 Debian-Security:9/oldoldstable [amd64])
func (deb *DebianPackageManager) parseUpdateAvailableLine(line string) *Package {
	match := debPkgUpdateRE.FindStringSubmatch(line)
	if len(match) == 0 {
		return nil
	}

	return &Package{
		Name:         match[1],
		Version:      match[2],
		Architecture: match[4],
		Update:       match[3],
	}
}

var aptGetBaseCommand = []string{
	aptGetPath,
	`-o Dpkg::Options::="--force-confdef"`,
	`-o Dpkg::Options::="--force-confold"`,
	"-f",
	"-y",
}

// UpgradeAll performs system upgrade if there are available upgrades.
// On success, return number of packages upgraded, output of the upgrade command and nil error.
func (deb *DebianPackageManager) UpgradeAll(ctx context.Context, elevationCmd []string) (int, []byte, error) {
	// check for updates
	inventory, err := deb.ListPackages(ctx, elevationCmd)
	if err != nil {
		return 0, nil, err
	}

	deb.lock.Lock()
	defer deb.lock.Unlock()

	updatesAvailable := 0

	for _, pkg := range inventory {
		if pkg.Update != "" {
			updatesAvailable++
		}
	}

	if updatesAvailable == 0 {
		return 0, nil, nil
	}

	// perform system upgrade

	upgradeCommand := append(aptGetBaseCommand, "upgrade")
	distUpgradeCommand := append(aptGetBaseCommand, "dist-upgrade")

	aptUpgradeCmd, err := newAptGetCommand(ctx, elevationCmd, upgradeCommand)
	if err != nil {
		return 0, nil, err
	}

	aptDistUpgradeCmd, err := newAptGetCommand(ctx, elevationCmd, distUpgradeCommand)
	if err != nil {
		return 0, nil, err
	}

	// we are about to change the package database, so clear the cache on return
	defer func() {
		cache.Delete(debianPackagesCacheKey)
	}()

	upgradeOutput, err := utils.RunCommandOutput(aptUpgradeCmd)
	if err != nil {
		return 0, upgradeOutput, err
	}

	distUpgradeOutput, err := utils.RunCommandOutput(aptDistUpgradeCmd)
	if err != nil {
		return 0, distUpgradeOutput, err
	}

	combinedOutput := append(upgradeOutput, distUpgradeOutput...)

	return updatesAvailable, combinedOutput, nil
}

// Install ensures a package with provided version number is installed in the system.
// If version is empty, the latest version of the package is installed.
// Returns output of the installation command.
func (deb *DebianPackageManager) Install(ctx context.Context, pkgName, version string, elevationCmd []string) ([]byte, error) {
	deb.lock.Lock()
	defer deb.lock.Unlock()

	if version != "" {
		pkgName = fmt.Sprintf("%s=%s", pkgName, version)
	}

	var downgradesFlag string
	if deb.supportsAllowDowngradesFlag {
		downgradesFlag = "--allow-downgrades"
	} else {
		downgradesFlag = "--force-yes"
	}

	installCommand := append(aptGetBaseCommand, downgradesFlag, "install", pkgName)

	aptCmd, err := newAptGetCommand(ctx, elevationCmd, installCommand)
	if err != nil {
		return nil, err
	}

	defer cache.Delete(debianPackagesCacheKey)

	return utils.RunCommandOutput(aptCmd)
}

// InstallLocal package.
func (deb *DebianPackageManager) InstallLocal(ctx context.Context, pkgFilePath string, elevationCmd []string) ([]byte, error) {
	deb.lock.Lock()
	defer deb.lock.Unlock()

	defer cache.Delete(debianPackagesCacheKey)

	installCommand := []string{dpkgPath, "-i", pkgFilePath}
	dpkgOutput, err := utils.RunPrivilegedCommand(ctx, elevationCmd, installCommand)

	// dpkg succeeded, return
	if err == nil {
		return dpkgOutput, nil
	}

	// dpkg fails, so we need to run "apt-get install -f" to install any possible dependencies
	dpkgOutput = []byte(err.Error())
	installCommand = append(aptGetBaseCommand, "install")

	aptCmd, err := newAptGetCommand(ctx, elevationCmd, installCommand)
	if err != nil {
		return dpkgOutput, err
	}

	aptCmd.Env = append(aptCmd.Env, "DEBIAN_FRONTEND=noninteractive")

	aptOutput, err := utils.RunCommandOutput(aptCmd)
	return append(dpkgOutput, aptOutput...), err
}

// PackageArchitecture returns the architecture of the package manager
func (deb *DebianPackageManager) PackageArchitecture() (string, error) {

	if cachedArch, ok := cache.Get(debianPkgArchCacheKey); ok {
		return cachedArch.(string), nil
	}

	cmd := []string{dpkgPath, "--print-architecture"}

	output, err := utils.RunCommand(context.Background(), cmd)
	if err != nil {
		return "", err
	}

	cache.Set(debianPkgArchCacheKey, strings.TrimSpace(string(output)), pkgCacheTTL)

	return strings.TrimSpace(string(output)), nil
}

// ParsePackageFile parses package and return Package information.
func (deb *DebianPackageManager) ParsePackageFile(ctx context.Context, pkgFilePath string) (*Package, error) {
	cmd := []string{dpkgPath, "-I", pkgFilePath}

	pkg := new(Package)

	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil
		}

		switch fields[0] {
		case "Package:":
			pkg.Name = fields[1]
		case "Version:":
			pkg.Version = fields[1]
		case "Architecture:":
			pkg.Architecture = fields[1]
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if pkg.Name == "" || pkg.Version == "" || pkg.Architecture == "" {
		return nil, fmt.Errorf("invalid debian package: %s", pkgFilePath)
	}

	return pkg, nil
}

// IsSupportedArchitecture returns true if architecture is supported by the system
func (deb *DebianPackageManager) IsSupportedArchitecture(arch string) error {
	mainArch, err := deb.PackageArchitecture()
	if err != nil {
		return err
	}

	allArchs := []string{mainArch, "all"}
	for _, supportedArch := range allArchs {
		if supportedArch == arch {
			return nil
		}
	}

	return fmt.Errorf("architecture %s is not supported by the system", arch)
}

func newAptGetCommand(ctx context.Context, elevationCmd, cmd []string) (*exec.Cmd, error) {
	aptCmd, err := utils.NewPrivilegedCommand(ctx, elevationCmd, cmd)
	if err != nil {
		return nil, err
	}

	aptCmd.Env = append(aptCmd.Env, "DEBIAN_FRONTEND=noninteractive")
	return aptCmd, nil
}
