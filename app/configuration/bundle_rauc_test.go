package configuration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/image"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_Should_Generate_Rauc_Path(t *testing.T) {
	apiClient, _ := api.NewMockedClient()
	appDir := t.TempDir()
	cacheDir := t.TempDir()

	service := New(apiClient, appDir, cacheDir)
	service.configEndpointUnreachable = false
	service.WithURLSigner(new(mockURLSigner))

	raucBundle := RaucBundle{
		RaucBundle: "/path/to/bundle.raucb",
	}

	raucPath, err := raucBundle.resolveRaucPath(context.Background(), service)
	assert.NoError(t, err)
	assert.True(t, raucPath != "")

	service.configEndpointUnreachable = true
	raucPath, err = raucBundle.resolveRaucPath(context.Background(), service)
	assert.NoError(t, err)
	assert.True(t, raucPath == "")
}

func Test_Redact(t *testing.T) {

	var testCases = []struct {
		name       string
		raucBundle RaucBundle
		input      string
		raucPath   string
		expected   string
	}{
		{
			name: "downloaded bundle retains local install path",
			raucBundle: RaucBundle{
				RaucBundle: "https://example.com/bundle.raucb",
				Download:   true,
			},
			input:    "failed to install rauc bundle /var/cache/rauc/bundle.raucb: some error",
			raucPath: "/var/cache/rauc/bundle.raucb",
			expected: "failed to install rauc bundle /var/cache/rauc/bundle.raucb: some error",
		},
		{
			name: "streaming bundle replaces signed url with rauc bundle",
			raucBundle: RaucBundle{
				RaucBundle: "/path/to/bundle.raucb",
				Download:   false,
			},
			input:    "failed to install rauc bundle https://example.com/signed/bundle.raucb?token=abc: some error",
			raucPath: "https://example.com/signed/bundle.raucb?token=abc",
			expected: "failed to install rauc bundle /path/to/bundle.raucb: some error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.raucBundle.redact(tc.input, tc.raucPath)
			assert.Equal(t, result, tc.expected)
		})
	}
}

func Test_IsCompatible(t *testing.T) {

	var testCases = []struct {
		name        string
		raucVersion string
		expected    bool
	}{
		{
			"compatible with rauc in string",
			"rauc 1.10.1",
			true,
		},
		{
			"compatible without rauc in string",
			"1.10.1",
			true,
		},
		{
			"compatible without patch version",
			"1.10",
			true,
		},
		{
			"incompatible with rauc in string",
			"rauc 1.7.4",
			false,
		},
		{
			"incompatible without rauc in string",
			"1.7.4",
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ver := image.ParseRaucVersion(tc.raucVersion)
			assert.Equal(t, image.IsRaucCompatible(ver), tc.expected)
		})
	}

}

func Test_Before_Install(t *testing.T) {

	var raucStatus image.RaucStatus
	err := json.Unmarshal([]byte(StatusBeforeInstall), &raucStatus)
	assert.NoError(t, err)

	assert.Equal(t, raucStatus.Compatible, "qemu86-64 demo platform")

	// check the slots
	var bundleInfo RaucBundleInfo
	err = json.Unmarshal([]byte(BundleInfo), &bundleInfo)
	assert.NoError(t, err)

	assert.Equal(t, bundleInfo.Compatible, "qemu86-64 demo platform")

	doInstall, err := shouldInstall(&raucStatus, &bundleInfo, New(nil, "", t.TempDir()))
	assert.NoError(t, err)
	assert.True(t, doInstall)
}

func Test_Last_Install(t *testing.T) {
	var raucStatus image.RaucStatus
	err := json.Unmarshal([]byte(StatusBeforeInstall), &raucStatus)
	assert.NoError(t, err)

	var bundleInfo RaucBundleInfo
	err = json.Unmarshal([]byte(BundleInfo), &bundleInfo)
	assert.NoError(t, err)

	service := New(nil, "", t.TempDir())
	stateDir := raucStateDirectory(service)
	lastInstallFile := filepath.Join(stateDir, raucLastInstallFile)
	assert.NoError(t, writeLastInstall(stateDir, lastInstallFile, raucStatus.BootPrimary, &bundleInfo))

	doInstall, err := shouldInstall(&raucStatus, &bundleInfo, service)
	assert.NoError(t, err)
	assert.False(t, doInstall)

	otherBundle := bundleInfo
	otherBundle.Hash = "different"
	doInstall, err = shouldInstall(&raucStatus, &otherBundle, service)
	assert.NoError(t, err)
	assert.True(t, doInstall)
}

func Test_Last_Install_Invalidated_On_Boot_Primary_Change(t *testing.T) {
	var statusA image.RaucStatus
	assert.NoError(t, json.Unmarshal([]byte(StatusBeforeInstall), &statusA))

	var statusB image.RaucStatus
	assert.NoError(t, json.Unmarshal([]byte(StatusAfterReboot), &statusB))

	var bundleInfo RaucBundleInfo
	assert.NoError(t, json.Unmarshal([]byte(BundleInfo), &bundleInfo))

	service := New(nil, "", t.TempDir())
	stateDir := raucStateDirectory(service)
	lastInstallFile := filepath.Join(stateDir, raucLastInstallFile)

	// bundle installed into B while boot primary remained A
	assert.NoError(t, writeLastInstall(stateDir, lastInstallFile, statusA.BootPrimary, &bundleInfo))

	doInstall, err := shouldInstall(&statusA, &bundleInfo, service)
	assert.NoError(t, err)
	assert.False(t, doInstall)

	// booted and committed B, which already contains the bundle
	doInstall, err = shouldInstall(&statusB, &bundleInfo, service)
	assert.NoError(t, err)
	assert.False(t, doInstall)

	_, err = os.Stat(lastInstallFile)
	assert.True(t, os.IsNotExist(err))

	// rolled back to A, the bundle must be installed again
	doInstall, err = shouldInstall(&statusA, &bundleInfo, service)
	assert.NoError(t, err)
	assert.True(t, doInstall)
}

func Test_Last_Install_Legacy_Record(t *testing.T) {
	var raucStatus image.RaucStatus
	assert.NoError(t, json.Unmarshal([]byte(StatusBeforeInstall), &raucStatus))

	var bundleInfo RaucBundleInfo
	assert.NoError(t, json.Unmarshal([]byte(BundleInfo), &bundleInfo))

	service := New(nil, "", t.TempDir())
	stateDir := raucStateDirectory(service)
	lastInstallFile := filepath.Join(stateDir, raucLastInstallFile)

	// record without associated boot primary state cannot be trusted
	legacy, err := json.Marshal(bundleInfo)
	assert.NoError(t, err)
	assert.NoError(t, os.MkdirAll(stateDir, 0700))
	assert.NoError(t, os.WriteFile(lastInstallFile, legacy, 0600))

	doInstall, err := shouldInstall(&raucStatus, &bundleInfo, service)
	assert.NoError(t, err)
	assert.True(t, doInstall)

	_, err = os.Stat(lastInstallFile)
	assert.True(t, os.IsNotExist(err))
}

func Test_After_Reboot(t *testing.T) {

	var raucStatus image.RaucStatus
	err := json.Unmarshal([]byte(StatusAfterReboot), &raucStatus)
	assert.NoError(t, err)

	assert.Equal(t, raucStatus.Compatible, "qemu86-64 demo platform")

	// check the slots
	var bundleInfo RaucBundleInfo
	err = json.Unmarshal([]byte(BundleInfo), &bundleInfo)
	assert.NoError(t, err)

	assert.Equal(t, bundleInfo.Compatible, "qemu86-64 demo platform")

	doInstall, err := shouldInstall(&raucStatus, &bundleInfo, New(nil, "", t.TempDir()))
	assert.NoError(t, err)
	assert.False(t, doInstall)
}

func Test_Incompatible_Bundle(t *testing.T) {

	var raucStatus image.RaucStatus
	err := json.Unmarshal([]byte(StatusBeforeInstall), &raucStatus)
	assert.NoError(t, err)

	assert.Equal(t, raucStatus.Compatible, "qemu86-64 demo platform")

	// check the slots
	var bundleInfo RaucBundleInfo
	err = json.Unmarshal([]byte(BundleInfo), &bundleInfo)
	assert.NoError(t, err)

	assert.Equal(t, bundleInfo.Compatible, "qemu86-64 demo platform")

	bundleInfo.Compatible = "qemu86-64 demo platform 2"

	doInstall, err := shouldInstall(&raucStatus, &bundleInfo, New(nil, "", t.TempDir()))

	assert.True(t, err != nil)
	assert.Equal(t, err.Error(), "RAUC bundle 'qemu86-64 demo platform 2' is not compatible with the system 'qemu86-64 demo platform'")
	assert.False(t, doInstall)
}

var StatusBeforeInstall = `
{
	"compatible": "qemu86-64 demo platform",
	"variant": "",
	"booted": "A",
	"boot_primary": "rootfs.0",
	"slots": [
	  {
		"rootfs.1": {
		  "class": "rootfs",
		  "device": "/dev/sda5",
		  "type": "ext4",
		  "bootname": "B",
		  "state": "inactive",
		  "parent": null,
		  "mountpoint": null,
		  "boot_status": "bad",
		  "slot_status": {
			"bundle": {
			  "compatible": null
			}
		  }
		}
	  },
	  {
		"efi.0": {
		  "class": "efi",
		  "device": "/dev/sda",
		  "type": "boot-gpt-switch",
		  "bootname": null,
		  "state": "inactive",
		  "parent": null,
		  "mountpoint": null,
		  "boot_status": null,
		  "slot_status": {
			"bundle": {
			  "compatible": null
			}
		  }
		}
	  },
	  {
		"rescue.0": {
		  "class": "rescue",
		  "device": "/dev/sda3",
		  "type": "ext4",
		  "bootname": null,
		  "state": "inactive",
		  "parent": null,
		  "mountpoint": "/rescue",
		  "boot_status": null,
		  "slot_status": {
			"bundle": {
			  "compatible": null
			}
		  }
		}
	  },
	  {
		"rootfs.0": {
		  "class": "rootfs",
		  "device": "/dev/sda4",
		  "type": "ext4",
		  "bootname": "A",
		  "state": "booted",
		  "parent": null,
		  "mountpoint": "/",
		  "boot_status": "good",
		  "slot_status": {
			"bundle": {
			  "compatible": null
			}
		  }
		}
	  }
	]
  }
`

var StatusAfterReboot = `
{
	"compatible": "qemu86-64 demo platform",
	"variant": "",
	"booted": "B",
	"boot_primary": "rootfs.1",
	"slots": [
	  {
		"rootfs.1": {
		  "class": "rootfs",
		  "device": "/dev/sda5",
		  "type": "ext4",
		  "bootname": "B",
		  "state": "booted",
		  "parent": null,
		  "mountpoint": "/",
		  "boot_status": "good",
		  "slot_status": {
			"bundle": {
			  "compatible": "qemu86-64 demo platform",
			  "version": "1.0",
			  "description": "qemu-demo-bundle version 1.0-r0",
			  "build": "20240208072640",
			  "hash": "c8e058178ea59338ad973da76d36824c3aa859c512dff0deb563ec3f6dcd51d1"
			},
			"checksum": {
			  "sha256": "7a0e0631047fe41c338c0627b137945b6a120ee8b0809ada7f96f84f5832eae4",
			  "size": 236953600
			},
			"installed": {
			  "timestamp": "2024-02-08T14:49:27Z",
			  "count": 1
			},
			"activated": {
			  "timestamp": "2024-02-08T14:49:27Z",
			  "count": 1
			},
			"status": "ok"
		  }
		}
	  },
	  {
		"efi.0": {
		  "class": "efi",
		  "device": "/dev/sda",
		  "type": "boot-gpt-switch",
		  "bootname": null,
		  "state": "inactive",
		  "parent": null,
		  "mountpoint": null,
		  "boot_status": null,
		  "slot_status": {
			"bundle": {
			  "compatible": "qemu86-64 demo platform",
			  "version": "1.0",
			  "description": "qemu-demo-bundle version 1.0-r0",
			  "build": "20240208072640",
			  "hash": "c8e058178ea59338ad973da76d36824c3aa859c512dff0deb563ec3f6dcd51d1"
			},
			"checksum": {
			  "sha256": "916a38036105853b1f874988c9c45e1c7fbbae2d21da6767c2955b60c6e5a219",
			  "size": 33572864
			},
			"installed": {
			  "timestamp": "2024-02-08T14:49:12Z",
			  "count": 1
			},
			"status": "ok"
		  }
		}
	  },
	  {
		"rescue.0": {
		  "class": "rescue",
		  "device": "/dev/sda3",
		  "type": "ext4",
		  "bootname": null,
		  "state": "inactive",
		  "parent": null,
		  "mountpoint": "/rescue",
		  "boot_status": null,
		  "slot_status": {
			"bundle": {
			  "compatible": null
			}
		  }
		}
	  },
	  {
		"rootfs.0": {
		  "class": "rootfs",
		  "device": "/dev/sda4",
		  "type": "ext4",
		  "bootname": "A",
		  "state": "inactive",
		  "parent": null,
		  "mountpoint": null,
		  "boot_status": "good",
		  "slot_status": {
			"bundle": {
			  "compatible": null
			}
		  }
		}
	  }
	]
  }
`

var BundleInfo = `
{
	"compatible": "qemu86-64 demo platform",
	"version": "1.0",
	"description": "qemu-demo-bundle version 1.0-r0",
	"build": "20240208072640",
	"hooks": [],
	"hash": "c8e058178ea59338ad973da76d36824c3aa859c512dff0deb563ec3f6dcd51d1",
	"images": [
	  {
		"efi": {
		  "variant": null,
		  "filename": "efi-boot.vfat",
		  "checksum": "916a38036105853b1f874988c9c45e1c7fbbae2d21da6767c2955b60c6e5a219",
		  "size": 33572864,
		  "hooks": [],
		  "adaptive": []
		}
	  },
	  {
		"rootfs": {
		  "variant": null,
		  "filename": "core-image-minimal-qemux86-64.ext4",
		  "checksum": "7a0e0631047fe41c338c0627b137945b6a120ee8b0809ada7f96f84f5832eae4",
		  "size": 236953600,
		  "hooks": [],
		  "adaptive": [
			"block-hash-index"
		  ]
		}
	  }
	]
  }
`

func Test_SimpleArtifactInstall(t *testing.T) {

	r := runner.NewRaucRunner(t)

	raucBundlePath := "file:///rauc/test-bundle-1.0.0.raucb"
	destinationPath := filepath.Join(r.GetStateDirectory(), "rauc", "bundle.raucb")

	raucConfig := RaucBundle{
		RaucBundle:   raucBundlePath,
		Download:     true,
		DownloadPath: destinationPath,
	}
	raucConfig.Enabled = true

	config := CommittedConfig{
		Bundles: []string{BundleRauc},
		BundleData: BundleData{
			Rauc: &raucConfig,
		},
	}

	expectedReports := []string{
		"[INFO] Successfully downloaded file " + raucBundlePath + " to " + destinationPath,
		"[INFO] RAUC bundle successfully installed '" + raucBundlePath + "'",
		"[WARN] Scheduling system reboot.",
	}

	reports, _ := ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, reports, expectedReports)

	r.MustExec("rm", "-f", destinationPath)

	reports, _ = ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, len(reports), 0)
}

func Test_InstallWithRebootCondition(t *testing.T) {
	r := runner.NewRaucRunner(t)

	raucBundlePath := "file:///rauc/test-bundle-1.0.0.raucb"
	destinationPath := filepath.Join(r.GetStateDirectory(), "rauc", "bundle.raucb")

	raucConfig := RaucBundle{
		RaucBundle:      raucBundlePath,
		Download:        true,
		DownloadPath:    destinationPath,
		RebootCondition: "/bin/false",
	}
	raucConfig.Enabled = true

	config := CommittedConfig{
		Bundles: []string{BundleRauc},
		BundleData: BundleData{
			Rauc: &raucConfig,
		},
	}

	expectedReports := []string{
		"[INFO] Successfully downloaded file " + raucBundlePath + " to " + destinationPath,
		"[INFO] RAUC bundle successfully installed '" + raucBundlePath + "'",
	}
	reports, _ := ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, reports, expectedReports)

	reports, _ = ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, len(reports), 0)
}

func Test_UpgradeNoReboot(t *testing.T) {

	r := runner.NewRaucRunner(t)

	raucBundlePath := "file:///rauc/test-bundle-1.0.0.raucb"
	destinationPath := filepath.Join(r.GetStateDirectory(), "rauc", "bundle.raucb")
	raucStatePath := filepath.Join(r.GetStateDirectory(), "app_workdir", "cache", "rauc", "state.json")

	raucConfig := RaucBundle{
		RaucBundle:      raucBundlePath,
		Download:        true,
		DownloadPath:    destinationPath,
		RebootCondition: "/bin/false",
	}
	raucConfig.Enabled = true

	config := CommittedConfig{
		Bundles: []string{BundleRauc},
		BundleData: BundleData{
			Rauc: &raucConfig,
		},
	}

	expectedReports := []string{
		"[INFO] Successfully downloaded file " + raucBundlePath + " to " + destinationPath,
		"[INFO] RAUC bundle successfully installed '" + raucBundlePath + "'",
	}

	reports, _ := ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, reports, expectedReports)

	raucUpgradeBundlePath := "file:///rauc/test-bundle-2.0.0.raucb"
	raucConfig.RaucBundle = raucUpgradeBundlePath
	expectedReports = []string{
		"[INFO] Successfully downloaded file " + raucUpgradeBundlePath + " to " + destinationPath,
		"[INFO] RAUC bundle successfully installed '" + raucUpgradeBundlePath + "'",
	}

	// rauc install changes ownership of the bundle to root when installing so we need to change it if
	// we are running as unprivileged user.
	if r.GetUnprivileged() {
		r.MustExec("chown", runner.UnprivilegedUser+":"+runner.UnprivilegedUser, destinationPath)
	}

	reports, _ = ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, reports, expectedReports)

	// check that bundle is not installed or downloaded again as long as we have the state file
	r.MustExec("rm", "-f", destinationPath)
	reports, _ = ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, len(reports), 0)

	// check that if we remove the state file, the bundle is not installed again. It will be downloaded again
	// because we need it to compare metadata.
	expectedReports = []string{
		"[INFO] Successfully downloaded file " + raucUpgradeBundlePath + " to " + destinationPath,
	}

	r.MustExec("rm", "-f", raucStatePath)
	reports, _ = ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, reports, expectedReports)
}

func Test_InstallBootPrimaryNotUpdated(t *testing.T) {
	r := runner.NewRaucRunner(t)

	// makes dummy-bootloader.sh ignore set-primary, so boot primary stays unchanged after install
	r.MustExec("touch", "/etc/rauc/primary-slot-state-override")

	raucBundlePath := "file:///rauc/test-bundle-1.0.0.raucb"
	destinationPath := filepath.Join(r.GetStateDirectory(), "rauc", "bundle.raucb")
	raucCacheDir := filepath.Join(r.GetStateDirectory(), "app_workdir", "cache", "rauc")
	raucStatePath := filepath.Join(raucCacheDir, "state.json")
	lastInstallPath := filepath.Join(raucCacheDir, raucLastInstallFile)

	raucConfig := RaucBundle{
		RaucBundle:      raucBundlePath,
		Download:        true,
		DownloadPath:    destinationPath,
		RebootCondition: "/bin/false",
	}
	raucConfig.Enabled = true

	config := CommittedConfig{
		Bundles: []string{BundleRauc},
		BundleData: BundleData{
			Rauc: &raucConfig,
		},
	}

	expectedReports := []string{
		"[INFO] Successfully downloaded file " + raucBundlePath + " to " + destinationPath,
		"[INFO] RAUC bundle successfully installed '" + raucBundlePath + "'",
	}

	reports, _ := ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, reports, expectedReports)

	var raucStatus image.RaucStatus
	assert.NoError(t, json.Unmarshal(r.MustExec("rauc", "status", "--output-format", "json"), &raucStatus))
	assert.Equal(t, raucStatus.BootPrimary, "appfs.0")

	// run again to verify that the installation does not repeat

	reports, _ = ExecuteTestConfigInDocker(r, config)
	assert.Empty(t, reports)

	r.MustExec("test", "-f", lastInstallPath)

	// force a re-download so the install decision isn't short-circuited by the download state
	r.MustExec("rm", "-f", destinationPath, raucStatePath)

	expectedReports = []string{
		"[INFO] Successfully downloaded file " + raucBundlePath + " to " + destinationPath,
	}

	reports, _ = ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, reports, expectedReports)
}
