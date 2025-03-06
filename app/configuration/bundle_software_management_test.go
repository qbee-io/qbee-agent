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

//go:build unix

package configuration_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_SoftwareManagementBundle_InstallPackageFromFile(t *testing.T) {

	tt := []struct {
		name     string
		filename string
		runner   *runner.Runner
	}{
		{
			name:     "deb",
			filename: "file:///apt-repo/repo/qbee-test_2.1.1_all.deb",
			runner:   runner.New(t),
		},
		{
			name:     "rpm",
			filename: "file:///yum-repo/repo/qbee-test-2.1.1-1.noarch.rpm",
			runner:   runner.NewRHELRunner(t),
		},
		{
			name:     "opkg",
			filename: "file:///opkg-repo/repo/qbee-test_2.1.1_all.ipk",
			runner:   runner.NewOpenWRTRunner(t),
		},
	}

	wg := sync.WaitGroup{}

	for _, test := range tt {

		wg.Add(1)
		go func(r *runner.Runner, filename string) {
			defer wg.Done()
			packages := []configuration.Software{
				{
					Package:     filename,
					ServiceName: "qbee-test",
				},
			}

			reports := executeSoftwareManagementBundle(r, packages)

			expectedReports := []string{
				fmt.Sprintf("[INFO] Successfully installed '%s'", filename),
				// since we are not installing systemctl on the test docker image, we will get the following warning
				fmt.Sprintf("[WARN] Required restart of '%s' cannot be performed", filename),
			}
			assert.Equal(t, reports, expectedReports)

			// check if package was correctly installed
			output := r.MustExec("qbee-test")
			assert.Equal(t, string(output), "2.1.1")
		}(test.runner, test.filename)
	}
	wg.Wait()
}

func Test_SoftwareManagementBundle_InstallPackageFromFile_WithConflicts(t *testing.T) {

	tt := []struct {
		name     string
		filename string
		runner   *runner.Runner
	}{
		{
			name:     "deb",
			filename: "file:///apt-repo/repo/qbee-test-conflicts_1.0.0_all.deb",
			runner:   runner.New(t),
		},
		{
			name:     "rpm",
			filename: "file:///yum-repo/repo/qbee-test-conflicts-1.0.0-1.noarch.rpm",
			runner:   runner.NewRHELRunner(t),
		},
	}

	wg := sync.WaitGroup{}

	for _, test := range tt {
		wg.Add(1)
		go func(r *runner.Runner, filename string) {
			defer wg.Done()
			packages := []configuration.Software{
				{
					Package: filename,
				},
			}

			reports := executeSoftwareManagementBundle(r, packages)
			expectedReports := []string{
				fmt.Sprintf("[ERR] Unable to install '%s'", filename),
			}
			assert.Equal(t, reports, expectedReports)
		}(test.runner, test.filename)
	}
	wg.Wait()
}

func Test_SoftwareManagementBundle_InstallPackageFromFile_WithDependencies(t *testing.T) {
	tt := []struct {
		name     string
		filename string
		runner   *runner.Runner
	}{
		{
			name:     "deb",
			filename: "file:///apt-repo/repo/qbee-test-dep_1.0.0_all.deb",
			runner:   runner.New(t),
		},
		{
			name:     "rpm",
			filename: "file:///yum-repo/repo/qbee-test-dep-1.0.0-1.noarch.rpm",
			runner:   runner.NewRHELRunner(t),
		},
		{
			name:     "opkg",
			filename: "file:///opkg-repo/repo/qbee-test-dep_1.0.0_all.ipk",
			runner:   runner.NewOpenWRTRunner(t),
		},
	}

	wg := sync.WaitGroup{}

	for _, test := range tt {
		wg.Add(1)
		go func(r *runner.Runner, filename string) {
			defer wg.Done()
			packages := []configuration.Software{
				{
					Package:     filename,
					ServiceName: "qbee-test-dep",
				},
			}

			// execute configuration bundles
			reports := executeSoftwareManagementBundle(r, packages)
			expectedReports := []string{
				fmt.Sprintf("[INFO] Successfully installed '%s'", filename),
				// since we are not installing systemctl on the test docker image, we will get the following warning
				fmt.Sprintf("[WARN] Required restart of '%s' cannot be performed", filename),
			}
			assert.Equal(t, reports, expectedReports)

			// check if package was correctly installed
			output := r.MustExec("qbee-test-dep")
			assert.Equal(t, string(output), "dep 1.0.0")
		}(test.runner, test.filename)
	}
	wg.Wait()
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
						Parameters: []configuration.TemplateParameter{
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
		"[WARN] Required restart of 'qbee-test' cannot be performed",
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
		"[WARN] Required restart of 'qbee-test' cannot be performed",
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
		"[INFO] Restarted service 'test'",
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

func Test_SoftwareManagementBundle_InstallPackage_Unsupported_Architecture(t *testing.T) {

	tt := []struct {
		name     string
		filename string
		runner   *runner.Runner
	}{
		{
			name:     "deb",
			filename: "file:///apt-repo/unsupported-archs/arch-test_1.0.0_unsupported.deb",
			runner:   runner.New(t),
		},
		{
			name:     "rpm",
			filename: "file:///yum-repo/unsupported-archs/arch-test-1.0.0-1.unsupported.rpm",
			runner:   runner.NewRHELRunner(t),
		},
		{
			name:     "opkg",
			filename: "file:///opkg-repo/unsupported-archs/arch-test_1.0.0_unsupported.ipk",
			runner:   runner.NewOpenWRTRunner(t),
		},
	}

	wg := sync.WaitGroup{}

	for _, test := range tt {
		wg.Add(1)
		go func(r *runner.Runner, filename string) {
			defer wg.Done()
			packages := []configuration.Software{
				{
					Package:     filename,
					ServiceName: "qbee-test-dep",
				},
			}

			// execute configuration bundles
			reports := executeSoftwareManagementBundle(r, packages)
			expectedReports := []string{
				"[ERR] Unable to determine supported architecture for package arch-test",
			}
			assert.Equal(t, reports, expectedReports)
		}(test.runner, test.filename)
	}
	wg.Wait()

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
