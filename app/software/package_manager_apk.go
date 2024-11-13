package software

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"go.qbee.io/agent/app/utils"
	"go.qbee.io/agent/app/utils/cache"
)

// PackageManagerTypeDebian is the type of the Debian package manager.
const PackageManagerTypeAPK PackageManagerType = "apk"
const apkFileSuffix string = ".apk"

var apkPackagesCacheKey = fmt.Sprintf("%s:%s:packages", pkgCacheKeyPrefix, PackageManagerTypeAPK)
var apkPkgArchCacheKey = fmt.Sprintf("%s:%s:arch", pkgCacheKeyPrefix, PackageManagerTypeAPK)

const (
	apkPath     = "apk"
	apkLockFile = "/var/lib/apk/world"
)

type ApkPackageManager struct {
	lock sync.Mutex
}

// Type returns the package manager type.
func (apk *ApkPackageManager) Type() PackageManagerType {
	return PackageManagerTypeAPK
}

// FileSuffix returns the file suffix for the package manager.
func (apk *ApkPackageManager) FileSuffix() string {
	return apkFileSuffix
}

// IsSupported returns true if package manager is supported by the host system.
func (apk *ApkPackageManager) IsSupported() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("apk")
	return err == nil
}

// Busy returns true if package manager is currently busy.
func (apk *ApkPackageManager) Busy() (bool, error) {
	return false, nil
}

// ListPackages returns a list of packages with available updates.
func (apk *ApkPackageManager) ListPackages(ctx context.Context) ([]Package, error) {
	apk.lock.Lock()
	defer apk.lock.Unlock()

	if cachedPackages, ok := cache.Get(apkPackagesCacheKey); ok {
		return cachedPackages.([]Package), nil
	}

	installedPackages, err := apk.listInstalledPackages(ctx)
	if err != nil {
		return nil, err
	}

	var availableUpdates map[string]string
	availableUpdates, err = apk.listAvailableUpdates(ctx)
	if err != nil {
		return nil, err
	}

	for i, pkg := range installedPackages {
		installedPackages[i].Update = availableUpdates[pkg.ID()]
	}

	cache.Set(apkPackagesCacheKey, installedPackages, pkgCacheTTL)

	return installedPackages, nil
}

var apkNameVersRE = regexp.MustCompile(`^([a-zA-Z0-9\-\_]+)-([0-9\.]+-r\d+).*$`)

func (apk *ApkPackageManager) listInstalledPackages(ctx context.Context) ([]Package, error) {
	cmd := []string{apkPath, "list", "--installed"}

	installedPackages := make([]Package, 0)

	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		// alpine-baselayout-3.6.5-r0 x86_64 {alpine-baselayout} (GPL-2.0-only) [installed]
		parts := strings.Split(line, " ")
		if len(parts) != 5 {
			return nil
		}

		matches := apkNameVersRE.FindStringSubmatch(parts[0])

		if len(matches) != 3 {
			return nil
		}

		pkg := Package{
			Name:         matches[1],
			Version:      matches[2],
			Architecture: parts[1],
		}

		installedPackages = append(installedPackages, pkg)
		return nil
	})

	if err != nil {
		return nil, err

	}

	return installedPackages, nil
}

func (apk *ApkPackageManager) listAvailableUpdates(ctx context.Context) (map[string]string, error) {
	cmd := []string{
		apkPath,
		"update",
	}

	if _, err := utils.RunCommand(ctx, cmd); err != nil {
		return nil, err
	}

	updates := make(map[string]string)

	cmd = []string{apkPath, "list", "--upgradeable"}

	// only process lines matching the following format:
	// Inst libsystemd0 [232-25+deb9u13] (232-25+deb9u14 Debian-Security:9/oldoldstable [amd64])
	err := utils.ForLinesInCommandOutput(ctx, cmd, func(line string) error {
		// apk-tools-2.14.4-r1 x86_64 {apk-tools} (GPL-2.0-only) [upgradable from: apk-tools-2.14.4-r0]

		parts := strings.Split(line, " ")

		matches := apkNameVersRE.FindStringSubmatch(parts[0])
		if len(matches) != 3 {
			return nil
		}

		pkg := Package{
			Name:         matches[1],
			Version:      matches[2],
			Architecture: parts[1],
		}

		updates[pkg.ID()] = matches[2]
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updates, nil

}

// UpgradeAll performs upgrade of all packages.
// On success, return number of packages upgraded, output of the upgrade command and nil error.
func (apk *ApkPackageManager) UpgradeAll(ctx context.Context) (int, []byte, error) {
	// check for updates
	inventory, err := apk.ListPackages(ctx)
	if err != nil {
		return 0, nil, err
	}

	apk.lock.Lock()
	defer apk.lock.Unlock()

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
		apkPath,
		"upgrade",
	}

	var output []byte
	if output, err = utils.RunCommand(ctx, upgradeCommand); err != nil {
		return 0, output, err
	}

	cache.Delete(apkPackagesCacheKey)

	return updatesAvailable, output, err
}

// Install ensures a package with provided version number is installed in the system.
func (apk *ApkPackageManager) Install(ctx context.Context, pkgName, version string) ([]byte, error) {
	apk.lock.Lock()
	defer apk.lock.Unlock()

	defer cache.Delete(apkPackagesCacheKey)

	if version != "" {
		pkgName = fmt.Sprintf("%s=%s", pkgName, version)
	}

	installCommand := []string{
		apkPath,
		"add",
		pkgName,
	}

	return utils.RunCommand(ctx, installCommand)
}

// InstallLocal package.
func (apk *ApkPackageManager) InstallLocal(ctx context.Context, pkgFilePath string) ([]byte, error) {
	apk.lock.Lock()
	defer apk.lock.Unlock()

	defer cache.Delete(apkPackagesCacheKey)

	installCmd := []string{
		apkPath,
		"add",
		pkgFilePath,
	}

	return utils.RunCommand(ctx, installCmd)
}

// PackageArchitecture returns the architecture of the package manager
func (apk *ApkPackageManager) PackageArchitecture() (string, error) {
	if cachedArch, ok := cache.Get(apkPkgArchCacheKey); ok {
		return cachedArch.(string), nil
	}

	cmd := []string{apkPath, "--print-arch"}

	output, err := utils.RunCommand(context.Background(), cmd)
	if err != nil {
		return "", err
	}

	cache.Set(apkPackagesCacheKey, strings.TrimSpace(string(output)), pkgCacheTTL)

	return strings.TrimSpace(string(output)), nil
}

// IsSupportedArchitecture returns true if architecture is supported by the system
func (apk *ApkPackageManager) IsSupportedArchitecture(arch string) error {
	apkArch, err := apk.PackageArchitecture()
	if err != nil {
		return err
	}

	if arch != apkArch {
		return fmt.Errorf("architecture %s is not supported by apk package manager", arch)
	}
	return nil
}

var apkParseOpkgPackageRE = regexp.MustCompile(`^([^_]+)_(\d[^_]+)_(\S+).apk$`)

// ParsePackageFile parses an apk package file and returns a Package struct.
func (opkg *ApkPackageManager) ParsePackageFile(ctx context.Context, pkgFilePath string) (*Package, error) {
	// opkg does not support parsing local packages, we need to parse the package filename instead
	// split the filename into parts

	matches := apkParseOpkgPackageRE.FindStringSubmatch(filepath.Base(pkgFilePath))

	if len(matches) == 0 {
		return nil, fmt.Errorf("error parsing apk package file: invalid filename")
	}

	return &Package{
		Name:         matches[1],
		Version:      matches[2],
		Architecture: matches[3],
	}, nil
}
