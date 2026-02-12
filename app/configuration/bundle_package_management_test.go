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

package configuration_test

import (
	"bytes"
	"sync"
	"testing"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_PackageManagement_PreCondition(t *testing.T) {
	r := runner.New(t)

	// by touch-ing a file on the file system as a pre-condition, we can check if the pre-condition was executed
	executePackageManagementBundle(r, configuration.PackageManagementBundle{
		PreCondition: "touch /tmp/pre-condition",
	})

	// check that the pre-condition was executed
	if _, err := r.Exec("ls", "/tmp/pre-condition"); err != nil {
		t.Fatalf("expected file not found: %v", err)
	}
}

func Test_PackageManagement_InstallPackage_PreConditionFailed(t *testing.T) {
	r := runner.New(t)

	// with pre-condition returning non-zero exit code, we shouldn't see test package installed
	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		PreCondition: "false",
		Packages:     []configuration.Package{{Name: "qbee-test"}},
	})

	// check that no reports are recorded
	assert.Empty(t, reports)

	// check that the test program is not installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	assert.Empty(t, installedVersion)
}

func Test_PackageManagement_InstallPackage_NoPrecondition(t *testing.T) {
	r := runner.New(t)

	// with empty pre-condition system should work as if the pre-condition is successful
	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		Packages: []configuration.Package{{Name: "qbee-test"}},
	})

	// check if a correct report is recorded
	expectedReports := []string{"[INFO] Package 'qbee-test' successfully installed."}
	assert.Equal(t, reports, expectedReports)

	// check that the newest version of test package is installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	assert.Equal(t, installedVersion, "2.1.1")
}

func Test_PackageManagement_InstallPackage_PreconditionSuccess(t *testing.T) {
	r := runner.New(t)

	// with condition returning zero exit code, package should be installed
	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		PreCondition: "true",
		Packages:     []configuration.Package{{Name: "qbee-test"}},
	})

	// check if a correct report is recorded
	expectedReports := []string{"[INFO] Package 'qbee-test' successfully installed."}
	assert.Equal(t, reports, expectedReports)

	// check that the newest version of test package is installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	assert.Equal(t, installedVersion, "2.1.1")
}

func Test_PackageManagement_InstallPackage_Downgrade(t *testing.T) {

	runners := []*runner.Runner{
		runner.New(t),
		runner.NewRHELRunner(t),
	}

	wg := sync.WaitGroup{}

	for _, r := range runners {
		wg.Add(1)
		go func(r *runner.Runner) {
			defer wg.Done()

			installNewestVersionOfTestPackage(r)

			// when specifying lower version, we should expect a downgrade operation
			reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
				Packages: []configuration.Package{{Name: "qbee-test", Version: "1.0.1"}},
			})

			// check if a correct report is recorded
			expectedReports := []string{"[INFO] Package 'qbee-test' successfully installed."}
			assert.Equal(t, reports, expectedReports)

			// check that the newest version of test package is installed
			installedVersion := checkInstalledVersionOfTestPackage(r)
			assert.Equal(t, installedVersion, "1.0.1")
		}(r)
	}
	wg.Wait()
}

func Test_PackageManagement_InstallPackage_UpdateWithEmptyVersion(t *testing.T) {

	runners := []*runner.Runner{
		runner.New(t),
		runner.NewRHELRunner(t),
	}

	wg := sync.WaitGroup{}

	for _, r := range runners {
		wg.Add(1)
		go func(r *runner.Runner) {
			defer wg.Done()
			installOlderVersionOfTestPackage(r)

			// when package has no version string, we assume that the latest version should be installed
			reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
				Packages: []configuration.Package{{Name: "qbee-test"}},
			})

			// check if a correct report is recorded
			expectedReports := []string{"[INFO] Package 'qbee-test' successfully installed."}
			assert.Equal(t, reports, expectedReports)

			// check that the newest version of test package is installed
			installedVersion := checkInstalledVersionOfTestPackage(r)
			assert.Equal(t, installedVersion, "2.1.1")
		}(r)
	}
	wg.Wait()
}

func Test_PackageManagement_InstallPackage_UpdateWithLatestVersion(t *testing.T) {

	runners := []*runner.Runner{
		runner.New(t),
		runner.NewRHELRunner(t),
	}

	wg := sync.WaitGroup{}

	for _, r := range runners {
		wg.Add(1)
		go func(r *runner.Runner) {
			defer wg.Done()
			installOlderVersionOfTestPackage(r)

			// when package has the 'latest' version string we should always update to the latest available version
			reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
				Packages: []configuration.Package{{Name: "qbee-test", Version: "latest"}},
			})

			// check if a correct report is recorded
			expectedReports := []string{"[INFO] Package 'qbee-test' successfully installed."}
			assert.Equal(t, reports, expectedReports)

			// check that the newest version of test package is installed
			installedVersion := checkInstalledVersionOfTestPackage(r)
			assert.Equal(t, installedVersion, "2.1.1")
		}(r)
	}
	wg.Wait()
}

func Test_PackageManagement_InstallPackage_UpdateWithReboot(t *testing.T) {

	runners := []*runner.Runner{
		runner.New(t),
		runner.NewRHELRunner(t),
	}

	wg := sync.WaitGroup{}

	for _, r := range runners {
		wg.Add(1)
		go func(r *runner.Runner) {
			defer wg.Done()
			installOlderVersionOfTestPackage(r)

			// when package has no version string, we assume that the latest version should be installed
			reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
				RebootMode: configuration.RebootAlways,
				Packages:   []configuration.Package{{Name: "qbee-test"}},
			})

			// check if a correct reboot warning report is recorded
			expectedReports := []string{
				"[INFO] Package 'qbee-test' successfully installed.",
				"[WARN] Scheduling system reboot.",
			}
			assert.Equal(t, reports, expectedReports)

			// check that the newest version of test package is installed
			installedVersion := checkInstalledVersionOfTestPackage(r)
			assert.Equal(t, installedVersion, "2.1.1")
		}(r)
	}
	wg.Wait()
}

func Test_PackageManagement_UpgradeAll(t *testing.T) {
	runners := []*runner.Runner{
		runner.New(t),
		runner.NewRHELRunner(t),
	}

	wg := sync.WaitGroup{}

	for _, r := range runners {
		wg.Add(1)
		go func(r *runner.Runner) {
			defer wg.Done()
			fullUpgrade(r)

			installOlderVersionOfTestPackage(r)

			reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
				FullUpgrade: true,
			})

			// check if a correct report is recorded
			expectedReports := []string{"[INFO] Full upgrade was successful - 1 packages updated."}
			assert.Equal(t, reports, expectedReports)

			// check that the newest version of test package is installed
			installedVersion := checkInstalledVersionOfTestPackage(r)
			assert.Equal(t, installedVersion, "2.1.1")
		}(r)
	}
	wg.Wait()
}

func Test_PackageManagement_UpgradeAll_WithReboot(t *testing.T) {
	runners := []*runner.Runner{
		runner.New(t),
		runner.NewRHELRunner(t),
	}

	wg := sync.WaitGroup{}

	for _, r := range runners {
		wg.Add(1)
		go func(r *runner.Runner) {
			defer wg.Done()
			fullUpgrade(r)

			installOlderVersionOfTestPackage(r)

			reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
				FullUpgrade: true,
				RebootMode:  configuration.RebootAlways,
			})

			// check if a correct reboot warning report is recorded
			expectedReports := []string{
				"[INFO] Full upgrade was successful - 1 packages updated.",
				"[WARN] Scheduling system reboot.",
			}
			assert.Equal(t, reports, expectedReports)

			// check that the newest version of test package is installed
			installedVersion := checkInstalledVersionOfTestPackage(r)
			assert.Equal(t, installedVersion, "2.1.1")
		}(r)
	}
	wg.Wait()
}

func Test_PackageManagement_UpgradeAll_WithRebootWithoutChanges(t *testing.T) {
	runners := []*runner.Runner{
		runner.New(t),
		runner.NewRHELRunner(t),
	}

	wg := sync.WaitGroup{}

	for _, r := range runners {
		wg.Add(1)
		go func(r *runner.Runner) {
			defer wg.Done()
			// ensure we have the latest updates
			fullUpgrade(r)

			reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
				FullUpgrade: true,
				RebootMode:  configuration.RebootAlways,
			})

			assert.Empty(t, reports)
		}(r)
	}
	wg.Wait()
}

// helper functions

// installNewestVersionOfTestPackage makes sure that the newest version of the test package is installed
func installNewestVersionOfTestPackage(r *runner.Runner) {

	r.MustExec(r.PackageInstallCommand("qbee-test", "")...)

	// check that the newest version of test package is indeed installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	if installedVersion != "2.1.1" {
		panic("expected newest version, got " + installedVersion)
	}
}

// installOlderVersionOfTestPackage makes sure that the older version of the test package is installed
func installOlderVersionOfTestPackage(r *runner.Runner) {
	r.MustExec(r.PackageInstallCommand("qbee-test", "1.0.1")...)

	// check that the newest version of test package is indeed installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	if installedVersion != "1.0.1" {
		panic("expected older version, got " + installedVersion)
	}
}

// fullUpgrade makes sure that the system is fully upgraded
func fullUpgrade(r *runner.Runner) {
	updateCommands := r.FullUpdateCommand()
	for _, cmd := range updateCommands {
		r.MustExec(cmd...)
	}
}

// executePackageManagementBundle is a helper method to quickly execute package management bundle.
// On success, it returns a slice of produced reports.
func executePackageManagementBundle(r *runner.Runner, bundle configuration.PackageManagementBundle) []string {
	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundlePackageManagement},
		BundleData: configuration.BundleData{
			PackageManagement: &bundle,
		},
	}

	config.BundleData.PackageManagement.Enabled = true

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)

	return reports
}

// checkInstalledVersionOfTestPackage returns a version of installed test package or empty string if not found.
func checkInstalledVersionOfTestPackage(r *runner.Runner) string {
	output, err := r.Exec("qbee-test", "--version")
	if err != nil {
		if bytes.Contains(output, []byte(`"qbee-test": executable file not found`)) {
			return ""
		}

		panic(err)
	}

	return string(output)
}
