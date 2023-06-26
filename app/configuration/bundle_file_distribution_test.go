package configuration_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"qbee.io/platform/api/frontend/client"
	"qbee.io/platform/test/assert"
	"qbee.io/platform/test/device"

	"github.com/qbee-io/qbee-agent/app/configuration"
)

func Test_FileDistributionBundle(t *testing.T) {
	r := device.New(t)
	r.Bootstrap()

	// upload a known debian package to the file manager
	pkgContents := r.ReadFile("/apt-repo/test_2.1.1.deb")
	pkgFilename := fmt.Sprintf("%s_%d.deb", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(pkgFilename, pkgContents)

	// commit config for the device
	bundle := configuration.FileDistributionBundle{
		Metadata: configuration.Metadata{Enabled: true},
		FileSets: []configuration.FileSet{
			{Files: []configuration.File{{Source: pkgFilename, Destination: "/tmp/test1"}}},
		},
	}

	assert.NoError(t, r.API.AddConfigurationChange(client.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleFileDistribution,
		Config:     bundle}))
	assert.NoError(t, r.API.CommitConfiguration("test commit"))

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))

	// execute configuration bundles
	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully downloaded file %[1]s to /tmp/test1",
			pkgFilename),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("md5sum", "/tmp/test1")
	assert.Equal(t, string(output), "8562ee4d61fba99c1525e85215cc59f3  /tmp/test1")
}

func Test_FileDistributionBundle_IsTemplate(t *testing.T) {
	r := device.New(t)
	r.Bootstrap()

	// upload a known debian package to the file manager
	fileContents := []byte("example: {{test-key}}, {{unknown-key}}, {{broken-tag")
	fileManagerPath := fmt.Sprintf("%s_%d.txt", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(fileManagerPath, fileContents)

	// commit config for the device
	bundle := configuration.FileDistributionBundle{
		Metadata: configuration.Metadata{Enabled: true},
		FileSets: []configuration.FileSet{
			{
				Files: []configuration.File{
					{Source: fileManagerPath, Destination: "/tmp/test1", IsTemplate: true},
				},
				TemplateParameters: []configuration.TemplateParameter{
					{Key: "test-key", Value: "test-value"},
				},
			},
		},
	}

	assert.NoError(t, r.API.AddConfigurationChange(client.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleFileDistribution,
		Config:     bundle}))
	assert.NoError(t, r.API.CommitConfiguration("test commit"))

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))

	// execute configuration bundles
	expectedReports := []string{
		fmt.Sprintf(
			"[INFO] Successfully downloaded file %[1]s to /var/lib/qbee/app_workdir/cache/file_distribution/%[1]s",
			fileManagerPath),
		fmt.Sprintf("[INFO] Successfully rendered template file %[1]s to /tmp/test1", fileManagerPath),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("cat", "/tmp/test1")
	assert.Equal(t, string(output), "example: test-value, {{unknown-key}}, {{broken-tag")
}

func Test_FileDistributionBundle_AfterCommand(t *testing.T) {
	r := device.New(t)
	r.Bootstrap()

	// upload a known debian package to the file manager
	pkgContents := r.ReadFile("/apt-repo/test_2.1.1.deb")
	pkgFilename := fmt.Sprintf("%s_%d.deb", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(pkgFilename, pkgContents)

	// commit config for the device
	bundle := configuration.FileDistributionBundle{
		Metadata: configuration.Metadata{Enabled: true},
		FileSets: []configuration.FileSet{
			{
				Files: []configuration.File{
					{Source: pkgFilename, Destination: "/tmp/test1"},
				},
				AfterCommand: "echo 'it worked!' > /tmp/test2",
			},
		},
	}

	assert.NoError(t, r.API.AddConfigurationChange(client.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleFileDistribution,
		Config:     bundle}))
	assert.NoError(t, r.API.CommitConfiguration("test commit"))

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))

	// execute configuration bundles
	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully downloaded file %[1]s to /tmp/test1",
			pkgFilename),
		"[INFO] Successfully executed after command",
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("md5sum", "/tmp/test1")
	assert.Equal(t, string(output), "8562ee4d61fba99c1525e85215cc59f3  /tmp/test1")

	output = r.MustExec("cat", "/tmp/test2")
	assert.Equal(t, string(output), "it worked!")
}

func Test_FileDistributionBundle_PreCondition_True(t *testing.T) {
	r := device.New(t)
	r.Bootstrap()

	// upload a known debian package to the file manager
	pkgContents := r.ReadFile("/apt-repo/test_2.1.1.deb")
	pkgFilename := fmt.Sprintf("%s_%d.deb", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(pkgFilename, pkgContents)

	// commit config for the device
	bundle := configuration.FileDistributionBundle{
		Metadata: configuration.Metadata{Enabled: true},
		FileSets: []configuration.FileSet{
			{
				Files: []configuration.File{
					{Source: pkgFilename, Destination: "/tmp/test1"},
				},
				PreCondition: "true",
			},
		},
	}

	assert.NoError(t, r.API.AddConfigurationChange(client.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleFileDistribution,
		Config:     bundle}))
	assert.NoError(t, r.API.CommitConfiguration("test commit"))

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))

	// execute configuration bundles
	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully downloaded file %[1]s to /tmp/test1",
			pkgFilename),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("md5sum", "/tmp/test1")
	assert.Equal(t, string(output), "8562ee4d61fba99c1525e85215cc59f3  /tmp/test1")
}

func Test_FileDistributionBundle_PreCondition_False(t *testing.T) {
	r := device.New(t)
	r.Bootstrap()

	// upload a known debian package to the file manager
	pkgContents := r.ReadFile("/apt-repo/test_2.1.1.deb")
	pkgFilename := fmt.Sprintf("%s_%d.deb", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(pkgFilename, pkgContents)

	// commit config for the device
	bundle := configuration.FileDistributionBundle{
		Metadata: configuration.Metadata{Enabled: true},
		FileSets: []configuration.FileSet{
			{
				Files: []configuration.File{
					{Source: pkgFilename, Destination: "/tmp/test1"},
				},
				PreCondition: "false",
			},
		},
	}

	assert.NoError(t, r.API.AddConfigurationChange(client.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleFileDistribution,
		Config:     bundle}))
	assert.NoError(t, r.API.CommitConfiguration("test commit"))

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))

	assert.Empty(t, reports)

	// check if file was created
	output := r.MustExec("ls", "/tmp/")
	assert.Equal(t, string(output), "")
}

func Test_FileDistributionBundle_Destination_Dirname_Exists(t *testing.T) {
	r := device.New(t)
	r.Bootstrap()

	destDir := "/tmp/"

	// upload a known debian package to the file manager
	pkgContents := r.ReadFile("/apt-repo/test_2.1.1.deb")
	pkgFilename := fmt.Sprintf("%s_%d.deb", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(pkgFilename, pkgContents)

	// commit config for the device
	bundle := configuration.FileDistributionBundle{
		Metadata: configuration.Metadata{Enabled: true},
		FileSets: []configuration.FileSet{
			{
				Files: []configuration.File{
					{Source: pkgFilename, Destination: destDir},
				},
				PreCondition: "true",
			},
		},
	}

	assert.NoError(t, r.API.AddConfigurationChange(client.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleFileDistribution,
		Config:     bundle}))
	assert.NoError(t, r.API.CommitConfiguration("test commit"))

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))

	// execute configuration bundles
	expectedReports := []string{
		fmt.Sprintf(
			"[INFO] Successfully downloaded file %[1]s to %s",
			pkgFilename,
			filepath.Join(destDir, pkgFilename),
		),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("md5sum", filepath.Join(destDir, pkgFilename))
	assert.Equal(t, string(output), fmt.Sprintf("8562ee4d61fba99c1525e85215cc59f3  %s", filepath.Join(destDir, pkgFilename)))
}

func Test_FileDistributionBundle_Destination_Regular_Path(t *testing.T) {
	r := device.New(t)
	r.Bootstrap()

	destFile := "/tmp/test_2.1.1.deb"

	// upload a known debian package to the file manager
	pkgContents := r.ReadFile("/apt-repo/test_2.1.1.deb")
	pkgFilename := fmt.Sprintf("%s_%d.deb", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(pkgFilename, pkgContents)

	// commit config for the device
	bundle := configuration.FileDistributionBundle{
		Metadata: configuration.Metadata{Enabled: true},
		FileSets: []configuration.FileSet{
			{
				Files: []configuration.File{
					{Source: pkgFilename, Destination: destFile},
				},
				PreCondition: "true",
			},
		},
	}

	assert.NoError(t, r.API.AddConfigurationChange(client.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleFileDistribution,
		Config:     bundle}))
	assert.NoError(t, r.API.CommitConfiguration("test commit"))

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))

	// execute configuration bundles
	expectedReports := []string{
		fmt.Sprintf(
			"[INFO] Successfully downloaded file %[1]s to %s",
			pkgFilename,
			destFile,
		),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("md5sum", destFile)
	assert.Equal(t, string(output), fmt.Sprintf("8562ee4d61fba99c1525e85215cc59f3  %s", destFile))
}

func Test_FileDistributionBundle_Destination_Dirname_NotExists(t *testing.T) {

	r := device.New(t)
	r.Bootstrap()

	destDir := "/tmp/doesnotexist/"

	// upload a known debian package to the file manager
	pkgContents := r.ReadFile("/apt-repo/test_2.1.1.deb")
	pkgFilename := fmt.Sprintf("%s_%d.deb", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(pkgFilename, pkgContents)

	// commit config for the device
	bundle := configuration.FileDistributionBundle{
		Metadata: configuration.Metadata{Enabled: true},
		FileSets: []configuration.FileSet{
			{
				Files: []configuration.File{
					{Source: pkgFilename, Destination: destDir},
				},
				PreCondition: "true",
			},
		},
	}

	assert.NoError(t, r.API.AddConfigurationChange(client.Change{
		NodeID:     r.DeviceID,
		BundleName: configuration.BundleFileDistribution,
		Config:     bundle}))
	assert.NoError(t, r.API.CommitConfiguration("test commit"))

	// execute configuration bundles
	reports, _ := configuration.ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))

	assert.Empty(t, reports)

	output := r.MustExec("ls", "/tmp/")
	assert.Equal(t, string(output), "")
}
