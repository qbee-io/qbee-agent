package configuration_test

import (
	"bytes"
	"testing"

	"github.com/qbee-io/qbee-agent/app/configuration"
	"github.com/qbee-io/qbee-agent/app/test"
)

func Test_PackageManagement_PreCondition(t *testing.T) {
	r := test.New(t)

	// by touch-ing a file on the file system as a pre-condition, we can check if the pre-condition was executed
	executePackageManagementBundle(r, configuration.PackageManagementBundle{
		PreCondition: "touch /pre-condition",
	})

	// check that the pre-condition was executed
	if _, err := r.Exec("ls", "/pre-condition"); err != nil {
		t.Fatalf("expected file not found: %v", err)
	}
}

func Test_PackageManagement_InstallPackage_PreConditionFailed(t *testing.T) {
	r := test.New(t)

	// with pre-condition returning non-zero exit code, we shouldn't see test package installed
	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		PreCondition: "false",
		Packages:     []configuration.Package{{Name: "qbee-test"}},
	})

	// check that no reports are recorded
	test.Empty(t, reports)

	// check that the test program is not installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	test.Empty(t, installedVersion)
}

func Test_PackageManagement_InstallPackage_NoPrecondition(t *testing.T) {
	r := test.New(t)

	// with empty pre-condition system should work as if the pre-condition is successful
	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		Packages: []configuration.Package{{Name: "qbee-test"}},
	})

	// check if a correct report is recorded
	expectedReports := []string{"[INFO] Package 'qbee-test' successfully installed."}
	test.Equal(t, reports, expectedReports)

	// check that the newest version of test package is installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	test.Equal(t, installedVersion, "2.1.1")
}

func Test_PackageManagement_InstallPackage_PreconditionSuccess(t *testing.T) {
	r := test.New(t)

	// with condition returning zero exit code, package should be installed
	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		PreCondition: "true",
		Packages:     []configuration.Package{{Name: "qbee-test"}},
	})

	// check if a correct report is recorded
	expectedReports := []string{"[INFO] Package 'qbee-test' successfully installed."}
	test.Equal(t, reports, expectedReports)

	// check that the newest version of test package is installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	test.Equal(t, installedVersion, "2.1.1")
}

func Test_PackageManagement_InstallPackage_Downgrade(t *testing.T) {
	r := test.New(t)

	installNewestVersionOfTestPackage(r)

	// when specifying lower version, we should expect a downgrade operation
	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		Packages: []configuration.Package{{Name: "qbee-test", Version: "1.0.1"}},
	})

	// check if a correct report is recorded
	expectedReports := []string{"[INFO] Package 'qbee-test' successfully installed."}
	test.Equal(t, reports, expectedReports)

	// check that the newest version of test package is installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	test.Equal(t, installedVersion, "1.0.1")
}

func Test_PackageManagement_InstallPackage_UpdateWithEmptyVersion(t *testing.T) {
	r := test.New(t)

	installOlderVersionOfTestPackage(r)

	// when package has no version string, we assume that the latest version should be installed
	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		Packages: []configuration.Package{{Name: "qbee-test"}},
	})

	// check if a correct report is recorded
	expectedReports := []string{"[INFO] Package 'qbee-test' successfully installed."}
	test.Equal(t, reports, expectedReports)

	// check that the newest version of test package is installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	test.Equal(t, installedVersion, "2.1.1")
}

func Test_PackageManagement_InstallPackage_UpdateWithLatestVersion(t *testing.T) {
	r := test.New(t)

	installOlderVersionOfTestPackage(r)

	// when package has the 'latest' version string we should always update to the latest available version
	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		Packages: []configuration.Package{{Name: "qbee-test", Version: "latest"}},
	})

	// check if a correct report is recorded
	expectedReports := []string{"[INFO] Package 'qbee-test' successfully installed."}
	test.Equal(t, reports, expectedReports)

	// check that the newest version of test package is installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	test.Equal(t, installedVersion, "2.1.1")
}

func Test_PackageManagement_InstallPackage_UpdateWithReboot(t *testing.T) {
	r := test.New(t)

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
	test.Equal(t, reports, expectedReports)

	// check that the newest version of test package is installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	test.Equal(t, installedVersion, "2.1.1")
}

func Test_PackageManagement_UpgradeAll(t *testing.T) {
	r := test.New(t)

	// ensure we have the latest updates
	r.MustExec("apt-get", "update")
	r.MustExec("apt-get", "upgrade", "-y")

	installOlderVersionOfTestPackage(r)

	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		FullUpgrade: true,
	})

	// check if a correct report is recorded
	expectedReports := []string{"[INFO] Full upgrade was successful - 1 packages updated."}
	test.Equal(t, reports, expectedReports)

	// check that the newest version of test package is installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	test.Equal(t, installedVersion, "2.1.1")
}

func Test_PackageManagement_UpgradeAll_WithReboot(t *testing.T) {
	r := test.New(t)

	// ensure we have the latest updates
	r.MustExec("apt-get", "update")
	r.MustExec("apt-get", "upgrade", "-y")

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
	test.Equal(t, reports, expectedReports)

	// check that the newest version of test package is installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	test.Equal(t, installedVersion, "2.1.1")
}

func Test_PackageManagement_UpgradeAll_WithRebootWithoutChanges(t *testing.T) {
	r := test.New(t)

	// ensure we have the latest updates
	r.MustExec("apt-get", "update")
	r.MustExec("apt-get", "upgrade", "-y")

	reports := executePackageManagementBundle(r, configuration.PackageManagementBundle{
		FullUpgrade: true,
		RebootMode:  configuration.RebootAlways,
	})

	test.Empty(t, reports)
}

// helper functions

// installNewestVersionOfTestPackage makes sure that the newest version of the test package is installed
func installNewestVersionOfTestPackage(r *test.Runner) {
	r.MustExec("apt", "install", "qbee-test")

	// check that the newest version of test package is indeed installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	if installedVersion != "2.1.1" {
		panic("expected newest version, got " + installedVersion)
	}
}

// installOlderVersionOfTestPackage makes sure that the older version of the test package is installed
func installOlderVersionOfTestPackage(r *test.Runner) {
	r.MustExec("apt", "install", "qbee-test=1.0.1")

	// check that the newest version of test package is indeed installed
	installedVersion := checkInstalledVersionOfTestPackage(r)
	if installedVersion != "1.0.1" {
		panic("expected older version, got " + installedVersion)
	}
}

// executePackageManagementBundle is a helper method to quickly execute package management bundle.
// On success, it returns a slice of produced reports.
func executePackageManagementBundle(r *test.Runner, bundle configuration.PackageManagementBundle) []string {
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
func checkInstalledVersionOfTestPackage(r *test.Runner) string {
	output, err := r.Exec("qbee-test", "--version")
	if err != nil {
		if bytes.Contains(output, []byte(`"qbee-test": executable file not found`)) {
			return ""
		}

		panic(err)
	}

	return string(output)
}