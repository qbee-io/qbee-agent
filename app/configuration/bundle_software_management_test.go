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
	"fmt"
	"testing"
	"time"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

const cacheDir = "/var/lib/qbee/app_workdir/cache"

func Test_SoftwareManagementBundle_InstallPackageFromFile(t *testing.T) {
	r := runner.New(t)

	pkgFilename := "file:///apt-repo/test_2.1.1.deb"

	packages := []configuration.Software{
		{
			Package:     pkgFilename,
			ServiceName: "qbee-test",
		},
	}

	reports := executeSoftwareManagementBundle(r, packages)
	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully installed '%s'", pkgFilename),
		// since we are not installing systemctl on the test docker image, we will get the following warning
		fmt.Sprintf("[WARN] Required restart of '%s' cannot be performed.", pkgFilename),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("qbee-test")
	assert.Equal(t, string(output), "2.1.1")
}

func Test_SoftwareManagementBundle_InstallPackageFromFile_WithConflicts(t *testing.T) {
	r := runner.New(t)

	pkgFilename := "file:///apt-repo/qbee-test-conflicts_1.0.0_all.deb"

	packages := []configuration.Software{
		{
			Package: pkgFilename,
		},
	}

	reports := executeSoftwareManagementBundle(r, packages)
	expectedReports := []string{
		fmt.Sprintf("[ERR] Unable to install '%s'", pkgFilename),
	}
	assert.Equal(t, reports, expectedReports)
}

func Test_SoftwareManagementBundle_InstallPackageFromFile_WithDependencies(t *testing.T) {
	r := runner.New(t)

	pkgFilename := "file:///apt-repo/test_dep_1.0.0.deb"

	packages := []configuration.Software{
		{
			Package:     pkgFilename,
			ServiceName: "qbee-test-dep",
		},
	}

	// execute configuration bundles
	reports := executeSoftwareManagementBundle(r, packages)
	expectedReports := []string{
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

	// upload a test file to the file manager
	fileContents := []byte("test\nkey: {{k1}} / {{k2}}")
	filename := fmt.Sprintf("%s_%d", t.Name(), time.Now().UnixNano())
	r.CreateFile("/"+filename, fileContents)

	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleParameters, configuration.BundleSoftwareManagement},
		BundleData: configuration.BundleData{
			Parameters: &configuration.ParametersBundle{
				Metadata: configuration.Metadata{Enabled: true},
				Parameters: []configuration.Parameter{
					{Key: "p1", Value: "param-value"},
				},
			},
			SoftwareManagement: &configuration.SoftwareManagementBundle{
				Metadata: configuration.Metadata{Enabled: true},
				Items: []configuration.Software{
					{
						Package: "qbee-test",
						ConfigFiles: []configuration.ConfigFile{
							{
								ConfigTemplate: "file:///" + filename,
								ConfigLocation: "/etc/config.test",
							},
						},
						Parameters: []configuration.ConfigFileParameter{
							{Key: "k1", Value: "test-value"},
							{Key: "k2", Value: "$(p1)"},
						},
					},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)

	expectedReports := []string{
		"[INFO] Successfully installed 'qbee-test'",
		fmt.Sprintf("[INFO] Successfully rendered template file file:///%s to /etc/config.test", filename),
		// since we are not installing systemctl on the test docker image, we will get the following warning
		"[WARN] Required restart of 'qbee-test' cannot be performed.",
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("qbee-test")
	assert.Equal(t, string(output), "2.1.1")

	// check that the config files is present and correct
	gotFileContents := r.ReadFile("/etc/config.test")
	expectedContents := "test\nkey: test-value / param-value"
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
