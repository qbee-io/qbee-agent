package configuration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"qbee.io/platform/services/device"
	"qbee.io/platform/test/assert"
	"qbee.io/platform/test/runner"
)

func Test_resolveParameters(t *testing.T) {
	hostname, err := os.Hostname()
	assert.NoError(t, err)

	tests := []struct {
		name       string
		parameters []Parameter
		secrets    []Parameter
		value      string
		want       string
	}{
		{
			name:       "no parameters",
			parameters: []Parameter{},
			value:      "example $(key)",
			want:       "example $(key)",
		},
		{
			name: "has parameter",
			parameters: []Parameter{
				{Key: "key", Value: "test-value"},
			},
			value: "example $(key)",
			want:  "example test-value",
		},
		{
			name: "has secret",
			secrets: []Parameter{
				{Key: "secret", Value: "test-secret"},
			},
			value: "example $(secret)",
			want:  "example test-secret",
		},
		{
			name: "match the same key twice",
			parameters: []Parameter{
				{Key: "key", Value: "test-value"},
			},
			value: "example $(key) - $(key)",
			want:  "example test-value - test-value",
		},
		{
			name: "match more than one key",
			parameters: []Parameter{
				{Key: "key1", Value: "test-value-1"},
				{Key: "key2", Value: "test-value-2"},
			},
			value: "example $(key1) - $(key2)",
			want:  "example test-value-1 - test-value-2",
		},
		{
			name: "unclosed key tag",
			parameters: []Parameter{
				{Key: "key", Value: "test-value"},
			},
			value: "example $(key remaining text",
			want:  "example $(key remaining text",
		},
		{
			name: "ending with $",
			parameters: []Parameter{
				{Key: "key", Value: "test-value"},
			},
			value: "example $",
			want:  "example $",
		},
		{
			name: "ending with $(",
			parameters: []Parameter{
				{Key: "key", Value: "test-value"},
			},
			value: "example $(",
			want:  "example $(",
		},
		{
			name:       "system variable",
			parameters: []Parameter{},
			value:      "example $(sys.host)",
			want:       "example " + hostname,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parametersBundle := ParametersBundle{
				Parameters: tt.parameters,
				Secrets:    tt.secrets,
			}

			ctx := parametersBundle.Context(context.Background())

			got := resolveParameters(ctx, tt.value)

			assert.Equal(t, got, tt.want)
		})
	}
}

func Test_FileDistributionWithParameters(t *testing.T) {
	r := runner.New(t)
	r.Bootstrap()

	// upload a known debian package to the file manager
	pkgContents := r.ReadFile("/apt-repo/test_2.1.1.deb")
	pkgFilename := fmt.Sprintf("%s_%d.deb", t.Name(), time.Now().UnixNano())
	r.UploadTempFile(pkgFilename, pkgContents)

	// commit config for the device
	_, err := r.API.CreateConfigurationChange(device.Change{
		NodeID:     r.DeviceID,
		BundleName: BundleParameters,
		Config: ParametersBundle{
			Metadata: Metadata{Enabled: true},
			Parameters: []Parameter{
				{Key: "plain", Value: "plainValue"},
			},
			Secrets: []Parameter{
				{Key: "secret", Value: "secretValue"},
			},
		},
	})
	assert.NoError(t, err)

	_, err = r.API.CreateConfigurationChange(device.Change{
		NodeID:     r.DeviceID,
		BundleName: BundleFileDistribution,
		Config: FileDistributionBundle{
			Metadata: Metadata{Enabled: true},
			FileSets: []FileSet{
				{
					Files: []File{
						{
							Source:      pkgFilename,
							Destination: "/$(plain)/$(secret).deb",
						},
					},
				},
			},
		},
	})
	assert.NoError(t, err)

	_, err = r.API.CommitConfiguration("test commit")
	assert.NoError(t, err)

	// execute configuration bundles
	reports, _ := ParseTestConfigExecuteOutput(r.MustExec("qbee-agent", "config", "-r"))

	// execute configuration bundles
	expectedReports := []string{
		fmt.Sprintf("[INFO] Successfully downloaded file %[1]s to /plainValue/********.deb",
			pkgFilename),
	}
	assert.Equal(t, reports, expectedReports)

	// check if package was correctly installed
	output := r.MustExec("md5sum", "/plainValue/secretValue.deb")
	assert.Equal(t, string(output), "8562ee4d61fba99c1525e85215cc59f3  /plainValue/secretValue.deb")
}
