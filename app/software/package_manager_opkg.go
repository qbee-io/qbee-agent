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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"go.qbee.io/agent/app/utils"
	"go.qbee.io/agent/app/utils/cache"
)

// PackageManagerTypeOpkg is the type of the opkg package manager.
const PackageManagerTypeOpkg PackageManagerType = "opkg"
const opkgFileSuffix string = ".ipk"

var opkgPackagesCacheKey = fmt.Sprintf("%s:%s:packages", pkgCacheKeyPrefix, PackageManagerTypeOpkg)
var opkgPkgArchCacheKey = fmt.Sprintf("%s:%s:arch", pkgCacheKeyPrefix, PackageManagerTypeOpkg)

const (
	opkgLockPath = "/var/lock/opkg.lock"
	opkgLockMode = 0640
	opkgCmd      = "opkg"
)

// OpkgPackageManager implements PackageManager interface for Opkg based systems (OpenWRT etc.)
type OpkgPackageManager struct {
	lock sync.Mutex
}

// Type returns type of the package manager.
func (opkg *OpkgPackageManager) Type() PackageManagerType {
	return PackageManagerTypeOpkg
}

// FileSuffix returns the file suffix for the package manager.
func (opkg *OpkgPackageManager) FileSuffix() string {
	return opkgFileSuffix
}

// IsSupported returns true if package manager is supported by the host system.
func (opkg *OpkgPackageManager) IsSupported() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	_, err := exec.LookPath(opkgCmd)
	return err == nil
}

// Busy returns true if opog is currently locked.
func (opkg *OpkgPackageManager) Busy() (bool, error) {
	opkg.lock.Lock()
	defer opkg.lock.Unlock()

	return checkPackageManagerLockFile(opkgLockPath, opkgLockMode)
}

// checkPackageManagerLockFile checks if the package manager lock file is locked.
// TODO: This function is duplicated in the Debian package manager. It should be moved to a common place.
func checkPackageManagerLockFile(lockPath string, lockMode os.FileMode) (bool, error) {
	// check the lock by attempting to acquire one
	file, err := os.OpenFile(lockPath, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC, lockMode)
	if err != nil {
		return false, fmt.Errorf("cannot open file %s: %w", lockPath, err)
	}

	defer file.Close()

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
func (opkg *OpkgPackageManager) ListPackages(ctx context.Context) ([]Package, error) {
	opkg.lock.Lock()
	defer opkg.lock.Unlock()

	if cachedPackages, ok := cache.Get(opkgPackagesCacheKey); ok {
		return cachedPackages.([]Package), nil
	}

	installedPackages, err := opkg.listInstalledPackages(ctx)
	if err != nil {
		return nil, fmt.Errorf("error listing installed packages: %w", err)
	}

	// availableUpdates = map[pkgName]updateVersion
	var availableUpdates map[string]string
	if availableUpdates, err = opkg.listAvailableUpdates(ctx); err != nil {
		return nil, fmt.Errorf("error listing available updates: %w", err)
	}

	for i, pkg := range installedPackages {
		updateVersion, ok := availableUpdates[pkg.Name]
		if ok {
			installedPackages[i].Update = updateVersion
		}
	}

	cache.Set(opkgPackagesCacheKey, installedPackages, pkgCacheTTL)

	return installedPackages, nil
}

func (opkg *OpkgPackageManager) listAvailableUpdates(ctx context.Context) (map[string]string, error) {

	updateCmd := []string{opkgCmd, "update"}

	if _, err := utils.RunCommand(ctx, updateCmd); err != nil {
		return nil, err
	}

	cmd := []string{opkgCmd, "list-upgradable"}
	updates := make(map[string]string)

	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		pkg := opkg.parseUpdateAvailableLine(line)
		if pkg != nil {
			updates[pkg.ID()] = pkg.Update
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error listing available updates: %w", err)
	}

	return updates, nil
}

var opkgPkgUpdateRE = regexp.MustCompile(`^(\S+)\s+-\s+(\S+)\s+-\s+(\S+)`)

func (opkg *OpkgPackageManager) parseUpdateAvailableLine(line string) *Package {
	matches := opkgPkgUpdateRE.FindStringSubmatch(line)

	if len(matches) == 0 {
		return nil
	}

	return &Package{
		Name:    matches[1],
		Version: matches[2],
		Update:  matches[3],
	}
}

const (
	opkgControlPath = "/usr/lib/opkg/info"
)

// listInstalledPackages returns a list of installed packages.
func (opkg *OpkgPackageManager) listInstalledPackages(ctx context.Context) ([]Package, error) {

	cmd := []string{opkgCmd, "list-installed"}

	installedPackages := make([]Package, 0)

	// only process lines matching the following format:
	// ii  libsystemd0:amd64           232-25+deb9u13     amd64              systemd utility library
	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {

		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil
		}

		pkg := Package{
			Name:    fields[0],
			Version: fields[2],
		}

		// resolve the architecture of the package
		arch, err := opkg.resolvePackageArchitecture(pkg.Name)
		if err != nil {
			return fmt.Errorf("error resolving package architecture: %w", err)
		}

		pkg.Architecture = arch

		installedPackages = append(installedPackages, pkg)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error listing installed packages: %w", err)
	}

	return installedPackages, nil
}

// resolvePackageArchitecture returns the architecture of a package from control file
// calling opkg info <package> is not efficient

var opkgControlArchitectureRE = regexp.MustCompile(`^Architecture:\s+(\S+)`)

func (opkg *OpkgPackageManager) resolvePackageArchitecture(packageName string) (string, error) {

	pkgControlFile := filepath.Join(opkgControlPath, packageName+".control")

	if _, err := os.Stat(pkgControlFile); err != nil {
		return "", fmt.Errorf("error reading package control file: %w", err)
	}

	// read the control file
	controlFile, err := os.Open(pkgControlFile)
	if err != nil {
		return "", fmt.Errorf("error opening package control file: %w", err)
	}
	defer controlFile.Close()

	// read the first 10 lines of the control file
	scanner := bufio.NewScanner(controlFile)

	for scanner.Scan() {
		if opkgControlArchitectureRE.MatchString(scanner.Text()) {
			return opkgControlArchitectureRE.FindStringSubmatch(scanner.Text())[1], nil
		}
	}

	return "", fmt.Errorf("error reading package control file: architecture not found")
}

// UpgradeAll performs upgrade of all packages.
func (opkg *OpkgPackageManager) UpgradeAll(ctx context.Context) (int, []byte, error) {
	// check for updates
	inventory, err := opkg.ListPackages(ctx)
	if err != nil {
		return 0, nil, err
	}

	opkg.lock.Lock()
	defer opkg.lock.Unlock()

	var cmdList [][]string
	for _, pkg := range inventory {
		if pkg.Update == "" {
			continue
		}
		cmdList = append(cmdList, []string{opkgCmd, "upgrade", pkg.Name})
	}

	if len(cmdList) == 0 {
		return 0, nil, nil
	}

	var output []byte

	for _, cmd := range cmdList {
		tmpOut, err := utils.RunCommand(ctx, cmd)
		output = append(output, tmpOut...)
		if err != nil {
			return 0, output, fmt.Errorf("error upgrading packages: %w", err)
		}
	}

	cache.Delete(opkgPackagesCacheKey)

	return len(cmdList), output, nil
}

// Install ensures a package with provided version number is installed in the system.
func (opkg *OpkgPackageManager) Install(ctx context.Context, pkgName, version string) ([]byte, error) {
	opkg.lock.Lock()
	defer opkg.lock.Unlock()

	cmd := []string{opkgCmd, "install", pkgName}
	if version != "" {
		return nil, fmt.Errorf("installing specific package versions is not supported by opkg")
	}

	defer cache.Delete(opkgPackagesCacheKey)

	return utils.RunCommand(ctx, cmd)
}

// InstallLocal package.
func (opkg *OpkgPackageManager) InstallLocal(ctx context.Context, pkgFilePath string) ([]byte, error) {
	opkg.lock.Lock()
	defer opkg.lock.Unlock()

	cmd := []string{opkgCmd, "install", pkgFilePath}

	defer cache.Delete(opkgPackagesCacheKey)

	return utils.RunCommand(ctx, cmd)
}

// PackageArchitecture returns the architecture of the package manager
func (opkg *OpkgPackageManager) PackageArchitecture() (string, error) {
	if cachedArch, ok := cache.Get(opkgPkgArchCacheKey); ok {
		return cachedArch.(string), nil
	}

	cmd := []string{opkgCmd, "print-architecture"}

	var arch string

	err := utils.ForLinesInCommandOutput(context.Background(), cmd, func(line string) error {
		fields := strings.Fields(line)

		if len(fields) < 3 {
			return nil
		}

		if fields[1] == "noarch" && fields[2] == "all" {
			return nil
		}
		arch = fields[1]
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error getting package architecture: %w", err)
	}

	cache.Set(opkgPkgArchCacheKey, arch, pkgCacheTTL)

	return arch, nil
}

var opkgParseOpkgPackageRE = regexp.MustCompile(`^([^_]+)_(\d[^_]+)_(\S+).ipk$`)

// ParsePackageFile parses an opkg package file and returns a Package struct.
func (opkg *OpkgPackageManager) ParsePackageFile(ctx context.Context, pkgFilePath string) (*Package, error) {
	// opkg does not support parsing local packages, we need to parse the package filename instead
	// split the filename into parts

	matches := opkgParseOpkgPackageRE.FindStringSubmatch(filepath.Base(pkgFilePath))

	if len(matches) == 0 {
		return nil, fmt.Errorf("error parsing opkg package file: invalid filename")
	}

	return &Package{
		Name:         matches[1],
		Version:      matches[2],
		Architecture: matches[3],
	}, nil
}
