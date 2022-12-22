package software

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/qbee-io/qbee-agent/app/utils"
)

const DebPackageManagerType PackageManagerType = "deb"

const (
	aptGetPath = "/usr/bin/apt-get"
	dpkgPath   = "/usr/bin/dpkg"
)

type DebPackageManager struct{}

// IsSupported returns true if package manager is supported by the host system.
func (deb *DebPackageManager) IsSupported() bool {
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

// ListPackages returns a list of packages with available updates.
func (deb *DebPackageManager) ListPackages() ([]Package, error) {
	installedPackages, err := deb.listInstalledPackages()
	if err != nil {
		return nil, fmt.Errorf("error listing installed packages: %w", err)
	}

	// availableUpdates = map[pkgName:arch]updateVersion
	var availableUpdates map[string]string
	if availableUpdates, err = deb.listAvailableUpdates(); err != nil {
		return nil, fmt.Errorf("error listing available updates: %w", err)
	}

	for i, pkg := range installedPackages {
		installedPackages[i].Update = availableUpdates[pkg.ID()]
	}

	return installedPackages, nil
}

// listInstalledPackages returns currently installed debian packages.
func (deb *DebPackageManager) listInstalledPackages() ([]Package, error) {
	cmd := []string{dpkgPath, "-l"}

	installedPackages := make([]Package, 0)

	// only process lines matching the following format:
	// ii  libsystemd0:amd64           232-25+deb9u13     amd64              systemd utility library
	err := utils.ForLinesInCommandOutput(cmd, func(line string) error {
		fields := strings.Fields(line)

		if fields[0] != "ii" || len(fields) < 4 {
			return nil
		}

		pkg := Package{
			Name:         strings.SplitN(fields[1], ":", 2)[0],
			Version:      fields[2],
			Architecture: fields[3],
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
func (deb *DebPackageManager) listAvailableUpdates() (map[string]string, error) {
	updateCmd := []string{aptGetPath, "update"}

	if _, err := utils.RunCommand(updateCmd); err != nil {
		return nil, err
	}

	updates := make(map[string]string)
	cmd := []string{aptGetPath, "--just-print", "--with-new-pkgs", "upgrade"}

	// only process lines matching the following format:
	// Inst libsystemd0 [232-25+deb9u13] (232-25+deb9u14 Debian-Security:9/oldoldstable [amd64])
	err := utils.ForLinesInCommandOutput(cmd, func(line string) error {
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

var debPkgUpdateRE = regexp.MustCompile("^Inst (\\S+) (?:\\[(.+)] )?\\((\\S+) .* \\[(.*)]\\)")

// parseUpdateAvailableLine parses a line from `apt-get --just-print upgrade` output into a Package.
// If line doesn't match the expected format, nil is returned.
// Supported format:
// Inst libsystemd0 [232-25+deb9u13] (232-25+deb9u14 Debian-Security:9/oldoldstable [amd64])
func (deb *DebPackageManager) parseUpdateAvailableLine(line string) *Package {
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
