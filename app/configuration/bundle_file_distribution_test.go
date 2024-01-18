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
	"path/filepath"
	"testing"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_FileDistributionBundle(t *testing.T) {
	r := runner.New(t)

	localFileRef := "file:///apt-repo/test_2.1.1.deb"

	agentConfig := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFileDistribution},
		BundleData: configuration.BundleData{
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{
					{Files: []configuration.File{{Source: localFileRef, Destination: "/tmp/test1"}}},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, agentConfig)

	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully downloaded file %[1]s to /tmp/test1",
			localFileRef),
	}
	assert.Equal(t, reports, expectedReports)

	output := r.MustExec("md5sum", "/tmp/test1")
	assert.Equal(t, string(output), "e45340c618b94c459663efc454ea1a50  /tmp/test1")
}

func Test_FileDistributionBundle_IsTemplate(t *testing.T) {
	r := runner.New(t)

	fileManagerPath := "/src.txt"
	r.CreateFile(fileManagerPath, []byte("example: {{test-key}}, {{unknown-key}}, {{broken-tag"))
	localFileRef := "file://" + fileManagerPath

	agentConfig := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFileDistribution},
		BundleData: configuration.BundleData{
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{
					{
						Files: []configuration.File{
							{Source: localFileRef, Destination: "/tmp/test1", IsTemplate: true},
						},
						TemplateParameters: []configuration.TemplateParameter{
							{Key: "test-key", Value: "test-value"},
						},
					},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, agentConfig)

	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully rendered template file %[1]s to /tmp/test1", localFileRef),
	}
	assert.Equal(t, reports, expectedReports)

	output := r.MustExec("cat", "/tmp/test1")
	assert.Equal(t, string(output), "example: test-value, {{unknown-key}}, {{broken-tag")
}

func Test_FileDistributionBundle_TemplateUsingParameters(t *testing.T) {
	r := runner.New(t)

	fileManagerPath := "/src.txt"
	r.CreateFile(fileManagerPath, []byte("example: {{test-param}}, {{test-secret}}"))
	localFileRef := "file://" + fileManagerPath

	agentConfig := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFileDistribution, configuration.BundleParameters},
		BundleData: configuration.BundleData{
			Parameters: &configuration.ParametersBundle{
				Metadata: configuration.Metadata{Enabled: true},
				Parameters: []configuration.Parameter{
					{Key: "param1", Value: "plain-text-value"},
				},
				Secrets: []configuration.Parameter{
					{Key: "secret1", Value: "secret-value"},
				},
			},
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{
					{
						Files: []configuration.File{
							{Source: localFileRef, Destination: "/tmp/test1", IsTemplate: true},
						},
						TemplateParameters: []configuration.TemplateParameter{
							{Key: "test-param", Value: "$(param1)"},
							{Key: "test-secret", Value: "$(secret1)"},
						},
					},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, agentConfig)

	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully rendered template file %[1]s to /tmp/test1", localFileRef),
	}
	assert.Equal(t, reports, expectedReports)

	output := r.MustExec("cat", "/tmp/test1")
	assert.Equal(t, string(output), "example: plain-text-value, secret-value")
}

func Test_FileDistributionBundle_AfterCommand(t *testing.T) {
	r := runner.New(t)

	localFileRef := "file:///apt-repo/test_2.1.1.deb"

	agentConfig := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFileDistribution},
		BundleData: configuration.BundleData{
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{
					{
						Files: []configuration.File{
							{Source: localFileRef, Destination: "/tmp/test1"},
						},
						AfterCommand: "echo 'it worked!' > /tmp/test2",
					},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, agentConfig)

	// execute configuration bundles
	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully downloaded file %[1]s to /tmp/test1", localFileRef),
		"[INFO] Successfully executed after command",
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("md5sum", "/tmp/test1")
	assert.Equal(t, string(output), "e45340c618b94c459663efc454ea1a50  /tmp/test1")

	output = r.MustExec("cat", "/tmp/test2")
	assert.Equal(t, string(output), "it worked!")
}

func Test_FileDistributionBundle_PreCondition_True(t *testing.T) {
	r := runner.New(t)

	localFileRef := "file:///apt-repo/test_2.1.1.deb"

	// commit config for the device
	agentConfig := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFileDistribution},
		BundleData: configuration.BundleData{
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{
					{
						Files: []configuration.File{
							{Source: localFileRef, Destination: "/tmp/test1"},
						},
						PreCondition: "true",
					},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, agentConfig)

	// execute configuration bundles
	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully downloaded file %[1]s to /tmp/test1",
			localFileRef),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("md5sum", "/tmp/test1")
	assert.Equal(t, string(output), "e45340c618b94c459663efc454ea1a50  /tmp/test1")
}

func Test_FileDistributionBundle_PreCondition_False(t *testing.T) {
	r := runner.New(t)

	localFileRef := "file:///apt-repo/test_2.1.1.deb"

	// commit config for the device
	agentConfig := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFileDistribution},
		BundleData: configuration.BundleData{
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{
					{
						Files: []configuration.File{
							{Source: localFileRef, Destination: "/tmp/test1"},
						},
						PreCondition: "false",
					},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, agentConfig)

	assert.Empty(t, reports)

	// check if file was created
	output := r.MustExec("ls", "/tmp/")
	assert.Equal(t, string(output), "")
}

func Test_FileDistributionBundle_Destination_Dirname_Exists(t *testing.T) {
	r := runner.New(t)

	destDir := "/tmp/"
	filename := "test_2.1.1.deb"
	localFileRef := "file:///apt-repo/" + filename

	agentConfig := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFileDistribution},
		BundleData: configuration.BundleData{
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{
					{
						Files: []configuration.File{
							{Source: localFileRef, Destination: destDir},
						},
					},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, agentConfig)

	expectedReports := []string{
		fmt.Sprintf(
			"[INFO] Successfully downloaded file %[1]s to %s",
			localFileRef,
			filepath.Join(destDir, filename),
		),
	}
	assert.Equal(t, reports, expectedReports)

	output := r.MustExec("md5sum", filepath.Join(destDir, filename))
	assert.Equal(t, string(output), fmt.Sprintf("e45340c618b94c459663efc454ea1a50  %s", filepath.Join(destDir, filename)))
}

func Test_FileDistributionBundle_Destination_Regular_Path(t *testing.T) {
	r := runner.New(t)

	localFileRef := "file:///apt-repo/test_2.1.1.deb"
	destFile := "/tmp/test_2.1.1.deb"

	agentConfig := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFileDistribution},
		BundleData: configuration.BundleData{
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{
					{
						Files: []configuration.File{
							{Source: localFileRef, Destination: destFile},
						},
						PreCondition: "true",
					},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, agentConfig)

	expectedReports := []string{
		fmt.Sprintf(
			"[INFO] Successfully downloaded file %[1]s to %s",
			localFileRef,
			destFile,
		),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("md5sum", destFile)
	assert.Equal(t, string(output), fmt.Sprintf("e45340c618b94c459663efc454ea1a50  %s", destFile))
}

func Test_FileDistributionBundle_Destination_Dirname_NotExists(t *testing.T) {
	r := runner.New(t)

	localFileRef := "file:///apt-repo/test_2.1.1.deb"
	destDir := "/tmp/doesnotexist/"

	agentConfig := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFileDistribution},
		BundleData: configuration.BundleData{
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{
					{
						Files: []configuration.File{
							{Source: localFileRef, Destination: destDir},
						},
					},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, agentConfig)

	assert.Empty(t, reports)

	output := r.MustExec("ls", "/tmp/")
	assert.Equal(t, string(output), "")
}

func Test_FileDistributionBundle_Destination_Is_Empty(t *testing.T) {
	r := runner.New(t)

	localFileRef := "file:///apt-repo/test_2.1.1.deb"
	destDir := ""

	agentConfig := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleFileDistribution},
		BundleData: configuration.BundleData{
			FileDistribution: &configuration.FileDistributionBundle{
				Metadata: configuration.Metadata{Enabled: true},
				FileSets: []configuration.FileSet{
					{
						Files: []configuration.File{
							{Source: localFileRef, Destination: destDir},
						},
					},
				},
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, agentConfig)

	assert.Empty(t, reports)
}
