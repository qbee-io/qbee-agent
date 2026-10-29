package configuration

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

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

	doInstall, err := shouldInstall(&raucStatus, &bundleInfo, noBootTime(t))
	assert.NoError(t, err)
	assert.True(t, doInstall)
}

// fixedBootTime returns a boot time resolver for the given boot time.
func fixedBootTime(bootTime time.Time) func() (time.Time, error) {
	return func() (time.Time, error) { return bootTime, nil }
}

// noBootTime returns a boot time resolver which fails the test if called.
func noBootTime(t *testing.T) func() (time.Time, error) {
	return func() (time.Time, error) {
		t.Fatal("boot time should not be resolved")
		return time.Time{}, nil
	}
}

func pendingActivation(t *testing.T, raucStatus *image.RaucStatus, class, hash string, bootTime time.Time) bool {
	t.Helper()
	pending, err := isBundlePendingActivation(raucStatus, class, hash, fixedBootTime(bootTime))
	assert.NoError(t, err)
	return pending
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

	doInstall, err := shouldInstall(&raucStatus, &bundleInfo, noBootTime(t))
	assert.NoError(t, err)
	assert.False(t, doInstall)

	// efi.0 holds the bundle but is not in the primary slot class, and the booted rootfs.1 is never considered
	bootTime := time.Date(2024, 2, 8, 14, 0, 0, 0, time.UTC)
	assert.False(t, pendingActivation(t, &raucStatus, "rootfs", bundleInfo.Hash, bootTime))
	assert.True(t, pendingActivation(t, &raucStatus, "efi", bundleInfo.Hash, bootTime))
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

	doInstall, err := shouldInstall(&raucStatus, &bundleInfo, noBootTime(t))

	assert.True(t, err != nil)
	assert.Equal(t, err.Error(), "RAUC bundle 'qemu86-64 demo platform 2' is not compatible with the system 'qemu86-64 demo platform'")
	assert.False(t, doInstall)
}

func Test_After_Install_Primary_Not_Updated(t *testing.T) {
	var raucStatus image.RaucStatus
	err := json.Unmarshal([]byte(StatusAfterInstallPrimaryNotUpdated), &raucStatus)
	assert.NoError(t, err)
	assert.Equal(t, raucStatus.BootPrimary, "rootfs.0")

	var bundleInfo RaucBundleInfo
	err = json.Unmarshal([]byte(BundleInfo), &bundleInfo)
	assert.NoError(t, err)

	installedAt := time.Date(2024, 2, 8, 14, 49, 27, 0, time.UTC)

	// bundle installed to rootfs.1 since boot, but boot_primary is not updated until reboot
	bootTime := installedAt.Add(-time.Hour)
	assert.True(t, pendingActivation(t, &raucStatus, "rootfs", bundleInfo.Hash, bootTime))

	doInstall, err := shouldInstall(&raucStatus, &bundleInfo, fixedBootTime(bootTime))
	assert.NoError(t, err)
	assert.False(t, doInstall)

	// system rebooted after install without switching boot_primary, so rootfs.1 is available for installation
	bootTime = installedAt.Add(time.Hour)
	doInstall, err = shouldInstall(&raucStatus, &bundleInfo, fixedBootTime(bootTime))
	assert.NoError(t, err)
	assert.True(t, doInstall)

	// another bundle was installed to rootfs.1 since boot
	for _, slot := range raucStatus.Slots {
		if slotData, ok := slot["rootfs.1"]; ok {
			slotData.SlotStatus.Bundle.Hash = "0000000000000000000000000000000000000000000000000000000000000000"
			slot["rootfs.1"] = slotData
		}
	}
	doInstall, err = shouldInstall(&raucStatus, &bundleInfo, noBootTime(t))
	assert.NoError(t, err)
	assert.True(t, doInstall)
}

func Test_After_Manual_Fallback(t *testing.T) {
	var raucStatus image.RaucStatus
	err := json.Unmarshal([]byte(StatusAfterManualFallback), &raucStatus)
	assert.NoError(t, err)
	assert.Equal(t, raucStatus.BootPrimary, "rootfs.0")
	assert.Equal(t, raucStatus.Booted, "A")

	var bundleInfo RaucBundleInfo
	err = json.Unmarshal([]byte(BundleInfo), &bundleInfo)
	assert.NoError(t, err)

	installedAt := time.Date(2024, 2, 8, 14, 49, 27, 0, time.UTC)

	// the user fell back to rootfs.0 and rebooted, so the boot time is newer than the install time of rootfs.1
	bootTime := time.Date(2024, 2, 9, 10, 0, 0, 0, time.UTC)
	assert.True(t, bootTime.After(installedAt))

	// rootfs.1 still holds the bundle, but it was installed before the last boot and is not pending activation
	assert.False(t, pendingActivation(t, &raucStatus, "rootfs", bundleInfo.Hash, bootTime))

	// the bundle is therefore considered for installation again
	doInstall, err := shouldInstall(&raucStatus, &bundleInfo, fixedBootTime(bootTime))
	assert.NoError(t, err)
	assert.True(t, doInstall)

	// had the system not booted since the install, rootfs.1 would be pending activation and installation is skipped
	doInstall, err = shouldInstall(&raucStatus, &bundleInfo, fixedBootTime(installedAt.Add(-time.Hour)))
	assert.NoError(t, err)
	assert.False(t, doInstall)
}

func Test_Pending_Activation_Multiple_Slots(t *testing.T) {
	var bundleInfo RaucBundleInfo
	err := json.Unmarshal([]byte(BundleInfo), &bundleInfo)
	assert.NoError(t, err)

	otherHash := "0000000000000000000000000000000000000000000000000000000000000000"

	var raucStatus image.RaucStatus
	err = json.Unmarshal([]byte(StatusThreeSlots), &raucStatus)
	assert.NoError(t, err)

	installSlot := func(slotName, hash string, installedAt time.Time) {
		for _, slot := range raucStatus.Slots {
			if slotData, ok := slot[slotName]; ok {
				slotData.SlotStatus.Bundle.Hash = hash
				slotData.SlotStatus.Installed.Timestamp = installedAt.Format(time.RFC3339)
				slot[slotName] = slotData
			}
		}
	}

	expectInstall := func(bootTime time.Time, expected bool) {
		t.Helper()
		doInstall, err := shouldInstall(&raucStatus, &bundleInfo, fixedBootTime(bootTime))
		assert.NoError(t, err)
		assert.Equal(t, doInstall, expected)
	}

	bootTime := time.Date(2024, 2, 8, 12, 0, 0, 0, time.UTC)

	// rootfs.2 has never been installed to and rootfs.1 holds another bundle
	installSlot("rootfs.1", otherHash, bootTime.Add(-time.Hour))
	expectInstall(bootTime, true)

	// the bundle is installed to rootfs.1 since boot and is pending activation
	installSlot("rootfs.1", bundleInfo.Hash, bootTime.Add(time.Hour))
	expectInstall(bootTime, false)

	// rootfs.2 holding another bundle does not affect the pending bundle in rootfs.1
	installSlot("rootfs.2", otherHash, bootTime.Add(-time.Hour))
	expectInstall(bootTime, false)

	// installing another bundle to rootfs.2 since boot does not affect the pending bundle in rootfs.1
	installSlot("rootfs.2", otherHash, bootTime.Add(2*time.Hour))
	expectInstall(bootTime, false)

	// the bundle in rootfs.1 is overwritten since boot, so it is no longer pending activation
	installSlot("rootfs.1", otherHash, bootTime.Add(3*time.Hour))
	expectInstall(bootTime, true)

	// the bundle is installed to rootfs.2 since boot and is pending activation
	installSlot("rootfs.2", bundleInfo.Hash, bootTime.Add(4*time.Hour))
	expectInstall(bootTime, false)

	// the system rebooted without activating the bundle, so it is installed again
	expectInstall(bootTime.Add(5*time.Hour), true)

	// a slot without install timestamp is never pending activation
	installSlot("rootfs.2", bundleInfo.Hash, bootTime.Add(4*time.Hour))
	for _, slot := range raucStatus.Slots {
		if slotData, ok := slot["rootfs.2"]; ok {
			slotData.SlotStatus.Installed.Timestamp = ""
			slot["rootfs.2"] = slotData
		}
	}
	expectInstall(bootTime, true)
}

// StatusThreeSlots has three slots of the rootfs class where rootfs.0 is booted and primary.
var StatusThreeSlots = `
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
		  "boot_status": "good",
		  "slot_status": {
			"bundle": {
			  "compatible": "qemu86-64 demo platform"
			},
			"installed": {
			  "count": 1
			},
			"status": "ok"
		  }
		}
	  },
	  {
		"rootfs.2": {
		  "class": "rootfs",
		  "device": "/dev/sda6",
		  "type": "ext4",
		  "bootname": "C",
		  "state": "inactive",
		  "boot_status": "good",
		  "slot_status": {
			"bundle": {
			  "compatible": "qemu86-64 demo platform"
			},
			"installed": {
			  "count": 1
			},
			"status": "ok"
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

// StatusAfterManualFallback emulates a system where the bundle was installed to and booted from rootfs.1,
// after which the user manually fell back to rootfs.0 (e.g. 'rauc status mark-active other') and rebooted.
var StatusAfterManualFallback = `
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
		  "boot_status": "good",
		  "slot_status": {
			"bundle": {
			  "compatible": "qemu86-64 demo platform",
			  "version": "1.0",
			  "description": "qemu-demo-bundle version 1.0-r0",
			  "build": "20240208072640",
			  "hash": "c8e058178ea59338ad973da76d36824c3aa859c512dff0deb563ec3f6dcd51d1"
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
			},
			"activated": {
			  "timestamp": "2024-02-09T09:55:00Z",
			  "count": 2
			}
		  }
		}
	  }
	]
  }
`

// StatusAfterInstallPrimaryNotUpdated emulates a bootloader backend which does not update boot_primary,
// bundle is installed to rootfs.1 but rootfs.0 is still primary.
var StatusAfterInstallPrimaryNotUpdated = `
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
		  "boot_status": "good",
		  "slot_status": {
			"bundle": {
			  "compatible": "qemu86-64 demo platform",
			  "version": "1.0",
			  "description": "qemu-demo-bundle version 1.0-r0",
			  "build": "20240208072640",
			  "hash": "c8e058178ea59338ad973da76d36824c3aa859c512dff0deb563ec3f6dcd51d1"
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

func Test_InstallPrimaryNotUpdated(t *testing.T) {
	r := runner.NewRaucRunner(t)

	// emulate a bootloader backend which does not update boot_primary until reboot
	r.MustExec("touch", "/etc/rauc/primary-slot-state-override")

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

	// remove the state file to force bundle metadata comparison, the bundle must not be installed again
	// since it was installed to the non-booted slot after the system booted
	if r.GetUnprivileged() {
		r.MustExec("chown", runner.UnprivilegedUser+":"+runner.UnprivilegedUser, destinationPath)
	}
	r.MustExec("rm", "-f", raucStatePath)

	reports, _ = ExecuteTestConfigInDocker(r, config)
	assert.Equal(t, len(reports), 0)
}
