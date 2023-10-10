package configuration_test

import (
	"fmt"
	"testing"
	"time"

	"qbee.io/platform/services/device"
	"qbee.io/platform/test/assert"
	"qbee.io/platform/test/runner"

	"github.com/qbee-io/qbee-agent/app/configuration"
)

const cacheDir = "/var/lib/qbee/app_workdir/cache"

func Test_SoftwareManagementBundle_InstallPackageFromFile(t *testing.T) {
	r := runner.New(t)
	r.Bootstrap()

	// upload a known debian package to the file manager
	pkgContents := r.ReadFile("/apt-repo/test_2.1.1.deb")
	pkgFilename := fmt.Sprintf("%s_%d.deb", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(pkgFilename, pkgContents)

	// commit config for the device
	bundle := configuration.SoftwareManagementBundle{
		Metadata: configuration.Metadata{Enabled: true},
		Items: []configuration.Software{
			{
				Package:     pkgFilename,
				ServiceName: "qbee-test",
			},
		},
	}

	_, err := r.API.CreateConfigurationChange(device.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleSoftwareManagement,
		Config:     bundle})
	assert.NoError(t, err)

	_, err = r.API.CommitConfiguration("test commit")
	assert.NoError(t, err)

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))
	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully downloaded file %[1]s to /var/lib/qbee/app_workdir/cache/software/%[1]s",
			pkgFilename),
		fmt.Sprintf("[INFO] Successfully installed '%s'", pkgFilename),
		// since we are not installing systemctl on the test docker image, we will get the following warning
		fmt.Sprintf("[WARN] Required restart of '%s' cannot be performed.", pkgFilename),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("qbee-test")
	assert.Equal(t, string(output), "2.1.1")
}

func Test_SoftwareManagementBundle_InstallPackageFromFile_WithDependencies(t *testing.T) {
	r := runner.New(t)
	r.Bootstrap()

	// upload a known debian package to the file manager
	pkgContents := r.ReadFile("/apt-repo/test_dep_1.0.0.deb")
	pkgFilename := fmt.Sprintf("%s_%d.deb", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(pkgFilename, pkgContents)

	// commit config for the device
	bundle := configuration.SoftwareManagementBundle{
		Metadata: configuration.Metadata{Enabled: true},
		Items: []configuration.Software{
			{
				Package:     pkgFilename,
				ServiceName: "qbee-test-dep",
			},
		},
	}

	_, err := r.API.CreateConfigurationChange(device.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleSoftwareManagement,
		Config:     bundle})
	assert.NoError(t, err)

	_, err = r.API.CommitConfiguration("test commit")
	assert.NoError(t, err)

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))
	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully downloaded file %[1]s to %[2]s/software/%[1]s", pkgFilename, cacheDir),
		fmt.Sprintf("[INFO] Successfully installed '%s'", pkgFilename),
		// since we are not installing systemctl on the test docker image, we will get the following warning
		fmt.Sprintf("[WARN] Required restart of '%s' cannot be performed.", pkgFilename),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("qbee-test-dep")
	assert.Equal(t, string(output), "dep 1.0.0")
}

func Test_SoftwareManagementBundle_InstallPackage_WithConfigFileTemplate(t *testing.T) {
	r := runner.New(t)
	r.Bootstrap()

	// upload a test file to the file manager
	fileContents := []byte("test\nkey: {{k1}}")
	filename := fmt.Sprintf("%s_%d", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(filename, fileContents)

	// commit config for the device
	bundle := configuration.SoftwareManagementBundle{
		Metadata: configuration.Metadata{Enabled: true},
		Items: []configuration.Software{
			{
				Package: "qbee-test",
				ConfigFiles: []configuration.ConfigFile{
					{
						ConfigTemplate: filename,
						ConfigLocation: "/etc/config.test",
					},
				},
				Parameters: []configuration.ConfigFileParameter{
					{
						Key:   "k1",
						Value: "test-value",
					},
				},
			},
		},
	}

	_, err := r.API.CreateConfigurationChange(device.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleSoftwareManagement,
		Config:     bundle})
	assert.NoError(t, err)

	_, err = r.API.CommitConfiguration("test commit")
	assert.NoError(t, err)

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))
	expectedReports := []string{
		"[INFO] Successfully installed 'qbee-test'",
		fmt.Sprintf("[INFO] Successfully downloaded file %[1]s to %[2]s/file_distribution/%[1]s", filename, cacheDir),
		fmt.Sprintf("[INFO] Successfully rendered template file %s to /etc/config.test", filename),
		// since we are not installing systemctl on the test docker image, we will get the following warning
		"[WARN] Required restart of 'qbee-test' cannot be performed.",
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("qbee-test")
	assert.Equal(t, string(output), "2.1.1")

	// check that the config files is present and correct
	gotFileContents := r.ReadFile("/etc/config.test")
	expectedContents := "test\nkey: test-value"
	assert.Equal(t, string(gotFileContents), expectedContents)
}

func Test_SoftwareManagementBundle_InstallPackage_RestartService_WithoutSystemctl(t *testing.T) {
	r := runner.New(t)

	// execute configuration bundles
	items := []configuration.Software{{Package: "qbee-test"}}

	reports := executeSoftwareManagementBundle(r, items)

	expectedReports := []string{
		"[INFO] Successfully installed 'qbee-test'",
		"[WARN] Required restart of 'qbee-test' cannot be performed.",
	}
	assert.Equal(t, reports, expectedReports)
}

func Test_SoftwareManagementBundle_InstallPackage_RestartService_NotAService(t *testing.T) {
	r := runner.New(t)

	// install systemctl
	r.MustExec("apt-get", "install", "-y", "systemctl")

	// execute configuration bundles
	items := []configuration.Software{{Package: "qbee-test"}}

	reports := executeSoftwareManagementBundle(r, items)

	expectedReports := []string{
		"[INFO] Successfully installed 'qbee-test'",
	}
	assert.Equal(t, reports, expectedReports)
}

func Test_SoftwareManagementBundle_InstallPackage_RestartService_NoServiceName(t *testing.T) {
	r := runner.New(t)

	// install systemctl
	r.MustExec("apt-get", "install", "-y", "systemctl")

	// execute configuration bundles
	items := []configuration.Software{{Package: "qbee-test-service"}}

	reports := executeSoftwareManagementBundle(r, items)

	expectedReports := []string{
		"[INFO] Successfully installed 'qbee-test-service'",
	}
	assert.Equal(t, reports, expectedReports)
}

func Test_SoftwareManagementBundle_InstallPackage_RestartService_WithServiceName(t *testing.T) {
	r := runner.New(t)

	// install systemctl
	r.MustExec("apt-get", "install", "-y", "systemctl")

	// execute configuration bundles
	items := []configuration.Software{
		{
			Package:     "qbee-test-service",
			ServiceName: "test",
		},
	}

	reports := executeSoftwareManagementBundle(r, items)

	expectedReports := []string{
		"[INFO] Successfully installed 'qbee-test-service'",
		"[INFO] Restarted service 'test'.",
	}
	assert.Equal(t, reports, expectedReports)
}

func Test_SoftwareManagementBundle_InstallPackage_PreCondition(t *testing.T) {
	testCases := []struct {
		name            string
		preCondition    string
		expectedReports []string
	}{
		{
			name:            "empty",
			preCondition:    "",
			expectedReports: []string{"[INFO] Successfully installed 'qbee-test-service'"},
		},
		{
			name:            "true",
			preCondition:    "true",
			expectedReports: []string{"[INFO] Successfully installed 'qbee-test-service'"},
		},
		{
			name:            "false",
			preCondition:    "false",
			expectedReports: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			r := runner.New(t)

			// install systemctl
			r.MustExec("apt-get", "install", "-y", "systemctl")

			// execute configuration bundles
			items := []configuration.Software{{Package: "qbee-test-service", PreCondition: testCase.preCondition}}

			reports := executeSoftwareManagementBundle(r, items)

			assert.Equal(t, reports, testCase.expectedReports)
		})
	}
}

// executeSoftwareManagementBundle is a helper method to quickly execute software management bundle.
// On success, it returns a slice of produced reports.
func executeSoftwareManagementBundle(r *runner.Runner, items []configuration.Software) []string {
	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleSoftwareManagement},
		BundleData: configuration.BundleData{
			SoftwareManagement: &configuration.SoftwareManagementBundle{
				Metadata: configuration.Metadata{Enabled: true},
				Items:    items,
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)

	return reports
}
