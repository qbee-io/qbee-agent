// Copyright 2024 qbee.io
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

package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"go.qbee.io/agent/app/image"
	"go.qbee.io/agent/app/inventory"
	"go.qbee.io/agent/app/utils"
)

// example payload
// {
//   "pre_condition": "true",
//   "rauc_bundle": "/path/to/bundle.raucb",
//   "download": true,
//   "download_path": "/tmp/bundle.raucb"
// }

// RaucBundle configures the system to install a RAUC bundle.
type RaucBundle struct {
	Metadata

	// PreCondition is a condition that must be met before the bundle is executed.
	PreCondition string `json:"pre_condition"`

	// RaucBundle is the path to the RAUC bundle file.
	RaucBundle string `json:"rauc_bundle"`

	// Download is a flag to indicate if the RAUC bundle should be downloaded.
	Download bool `json:"download"`

	// DownloadPath is the path where the RAUC bundle should be downloaded.
	DownloadPath string `json:"download_path"`

	// RebootCondition is a condition that must be met before the system is rebooted
	RebootCondition string `json:"reboot_condition"`
}

// defaultDownloadPath is the default path where the RAUC bundle is downloaded.
const defaultDownloadPath = "/tmp/bundle.raucb"

// Execute RAUC bundle configuration on the system.
func (r RaucBundle) Execute(ctx context.Context, service *Service) error {
	// check if the pre-condition is met

	if !image.HasRauc() {
		ReportError(ctx, nil, "RAUC not found")
		return nil
	}

	raucVersion, err := image.GetRaucVersion(ctx)

	if err != nil {
		ReportError(ctx, err, "Failed to get RAUC version")
		return err
	}

	isCompatible := image.IsRaucCompatible(raucVersion)
	if !isCompatible {
		ReportError(ctx, nil, "RAUC version '%s' is not compatible with the agent", raucVersion)
		return fmt.Errorf("RAUC version '%s' is not compatible with the agent", raucVersion)
	}

	if !CheckCondition(ctx, r.PreCondition) {
		return nil
	}

	raucStatus, err := image.GetRaucStatus(ctx)
	if err != nil {
		ReportError(ctx, err, "Failed to get RAUC status")
		return err
	}

	if raucStatus.BootPrimary == "" {
		ReportError(ctx, nil, "RAUC primary boot slot not found")
		return fmt.Errorf("RAUC primary boot slot not found")
	}

	r.RaucBundle = resolveParameters(ctx, r.RaucBundle)
	raucPath, err := r.resolveRaucPath(ctx, service)
	if err != nil {
		ReportError(ctx, err, "Failed to resolve RAUC bundle path")
		return err
	}

	// if no errors where return, but raucPath is empty, we can assume that the bundle is already installed
	// (file download) or that we do not have connectivity to the config endpoint (streaming). In these cases
	// we should return without doing anything
	if raucPath == "" {
		return nil
	}

	raucBundleInfo, err := r.getRaucBundleInfo(ctx, raucPath)
	if err != nil {
		ReportError(
			ctx,
			r.redact(err.Error(), raucPath),
			"Failed to get RAUC bundle info for '%s'", r.RaucBundle,
		)
		return err
	}

	if raucStatus.Compatible != raucBundleInfo.Compatible {
		ReportError(ctx, err, "RAUC bundle '%s' is not compatible with the system '%s'", raucBundleInfo.Compatible, raucStatus.Compatible)
		return err
	}

	// determine if the bundle should be installed based on the current boot time and RAUC status
	execInstall, err := shouldInstall(raucStatus, raucBundleInfo, func() (time.Time, error) {
		return getBootTime(ctx)
	})

	if err != nil {
		ReportError(ctx, err, "Failed to install RAUC bundle")
		return err
	}

	if !execInstall {
		return nil
	}

	raucInstallCmd := []string{"rauc", "install", raucPath}
	output, err := utils.RunPrivilegedCommand(ctx, raucInstallCmd)

	if err != nil {
		ReportError(
			ctx,
			r.redact(err.Error(), raucPath),
			"Failed to install RAUC bundle",
		)
		return err
	}

	ReportInfo(
		ctx,
		r.redact(string(output), raucPath),
		"RAUC bundle successfully installed '%s'",
		r.RaucBundle,
	)

	if CheckCondition(ctx, r.RebootCondition) {
		service.RebootAfterRun(ctx)
	}
	return nil
}

// RaucImageInfo represents the information about a RAUC image.
type RaucImageInfo struct {
	// Variant - RAUC image variant
	Variant string `json:"variant"`

	// Filename - RAUC image filename
	Filename string `json:"filename"`

	// Checksum - RAUC image checksum
	Checksum string `json:"checksum"`

	// Size - RAUC image size
	Size int64 `json:"size"`

	// Adaptive - RAUC image adaptive metadata
	Adaptive []string `json:"adaptive"`
}

// RaucBundleInfo represents the information about a RAUC bundle.
type RaucBundleInfo struct {
	// Compatible - RAUC compatible version.
	Compatible string `json:"compatible"`

	// Version - RAUC bundle version.
	Version string `json:"version"`

	// Description - RAUC bundle description.
	Description string `json:"description"`

	// Build - RAUC bundle build id/number.
	Build string `json:"build"`

	// Hash - RAUC bundle hash.
	Hash string `json:"hash"`

	// Images - RAUC bundle images metadata
	Images []map[string]RaucImageInfo `json:"images"`
}

// getRaucBundleInfo retrieves metadata (compatibility, version, hash, etc.) for the RAUC bundle at the given path/URL.
func (r RaucBundle) getRaucBundleInfo(ctx context.Context, url string) (*RaucBundleInfo, error) {

	raucInfoCmd := []string{"rauc", "info", "--output-format", "json", url}
	raucInfoBytes, err := utils.RunPrivilegedCommand(ctx, raucInfoCmd)

	if err != nil {
		return nil, err
	}

	var raucInfoBundle RaucBundleInfo
	err = json.Unmarshal(raucInfoBytes, &raucInfoBundle)
	if err != nil {
		return nil, err
	}
	return &raucInfoBundle, nil
}

// shouldInstall decides whether the given remote RAUC bundle needs to be installed on this system.
//
// The decision is made in three stages:
//  1. Compatibility check - reject bundles that were not built for this system.
//  2. "Already active" check (isBundleAlreadyActive) - is this exact bundle already running/primary?
//  3. "Already staged" check (isBundleStagedForNextBoot) - has it already been staged for next boot, i.e.
//     installed to the other slot since the last boot and just waiting for a reboot to become active?
//
// If either of the latter two checks is true, installing again would be redundant.
func shouldInstall(
	localRaucInfo *image.RaucStatus,
	remoteBundleData *RaucBundleInfo,
	getBootTime func() (time.Time, error),
) (bool, error) {

	// Check if the bundle is compatible with the system.
	if remoteBundleData.Compatible != localRaucInfo.Compatible {
		return false, fmt.Errorf("RAUC bundle '%s' is not compatible with the system '%s'", remoteBundleData.Compatible, localRaucInfo.Compatible)
	}

	// Resolve the slot RAUC considers primary/booted once, and share it with both checks below.
	_, currentSlotData, err := getPrimarySlot(localRaucInfo)
	if err != nil {
		return false, err
	}

	// Is this exact bundle already running/primary right now?
	if isBundleAlreadyActive(currentSlotData, remoteBundleData) {
		return false, nil
	}

	// The bundle isn't active, so check if it has already been staged for next boot instead.
	staged, err := isBundleStagedForNextBoot(localRaucInfo, currentSlotData, remoteBundleData, getBootTime)
	if err != nil {
		return false, err
	}

	// If the bundle is already staged for activation, no new install is needed; otherwise it must be installed.
	return !staged, nil
}

// isBundleAlreadyActive reports whether the remote bundle's hash matches the bundle currently running in the
// primary/booted slot, meaning it is already active and does not need to be (re-)installed.
func isBundleAlreadyActive(currentSlotData *image.SlotData, remoteBundleData *RaucBundleInfo) bool {
	return remoteBundleData.Hash == currentSlotData.SlotStatus.Bundle.Hash
}

// isBundleStagedForNextBoot reports whether the remote bundle has already been installed to a non-booted slot
// since the last boot, i.e. it is staged and will become active on the next reboot without needing a fresh
// install. Some bootloader backends do not update boot_primary until reboot, so this check is what catches
// that case even though the remote bundle isn't in the primary slot yet.
func isBundleStagedForNextBoot(
	localRaucInfo *image.RaucStatus,
	currentSlotData *image.SlotData,
	remoteBundleData *RaucBundleInfo,
	getBootTime func() (time.Time, error),
) (bool, error) {
	// If the primary slot reported by RAUC isn't actually the one that's booted, we're in an
	// inconsistent/transitional state and cannot reliably reason about staging - conservatively report
	// "not staged" so the caller falls back to installing.
	if currentSlotData.State != "booted" {
		return false, nil
	}

	return isBundlePendingActivation(localRaucInfo, currentSlotData.Class, remoteBundleData.Hash, getBootTime)
}

// isBundlePendingActivation returns true if any non-booted slot of the given class has been installed to since boot
// and holds the bundle with the given hash. Slots installed to before boot are not pending activation, since the
// system has booted in between without activating them.
func isBundlePendingActivation(
	localRaucInfo *image.RaucStatus,
	class, bundleHash string,
	getBootTime func() (time.Time, error),
) (bool, error) {
	bootTime, err := getBootTime()
	if err != nil {
		return false, err
	}

	for _, slot := range localRaucInfo.Slots {
		for _, slotData := range slot {
			if slotData.Class != class {
				continue
			}

			if slotData.State == "booted" {
				continue
			}

			if slotData.SlotStatus.Bundle.Hash != bundleHash {
				continue
			}

			if slotInstalledAt(&slotData).After(bootTime) {
				return true, nil
			}
		}
	}
	return false, nil
}

// slotInstalledAt returns the install timestamp of the slot, or the zero time if the slot has not been installed to.
func slotInstalledAt(slotData *image.SlotData) time.Time {
	installedAt, err := time.Parse(time.RFC3339, slotData.SlotStatus.Installed.Timestamp)
	if err != nil {
		return time.Time{}
	}
	return installedAt
}

// getBootTime returns the system boot time from the system inventory.
func getBootTime(ctx context.Context) (time.Time, error) {
	systemInventory, err := inventory.CollectSystemInventory(ctx)
	if err != nil {
		return time.Time{}, err
	}

	bootTime, err := strconv.ParseInt(systemInventory.System.BootTime, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("error parsing boot time '%s': %w", systemInventory.System.BootTime, err)
	}

	return time.Unix(bootTime, 0), nil
}

// getPrimarySlot returns the name and data of the slot RAUC currently reports as the primary/boot slot.
func getPrimarySlot(localRaucInfo *image.RaucStatus) (string, *image.SlotData, error) {
	for _, slot := range localRaucInfo.Slots {
		for slotName, slotData := range slot {
			if slotName == localRaucInfo.BootPrimary {
				return slotName, &slotData, nil
			}
		}
	}
	return "", nil, fmt.Errorf("no slot found in RAUC info")
}

// resolveRaucPath returns a local path or signed streaming URL for the configured RAUC bundle, downloading it first if requested.
func (r *RaucBundle) resolveRaucPath(ctx context.Context, service *Service) (string, error) {

	if r.Download {

		raucDownloadPath := defaultDownloadPath
		if r.DownloadPath != "" {
			raucDownloadPath = resolveParameters(ctx, r.DownloadPath)
		}

		return downloadRaucBundle(ctx, service, r.RaucBundle, raucDownloadPath)
	}
	return generateStreamingURL(service, r.RaucBundle)
}

// downloadRaucBundle downloads the remote RAUC bundle to raucDownloadPath if it isn't already cached there, and
// returns the local path, or an empty string if the bundle isn't available yet.
func downloadRaucBundle(ctx context.Context, service *Service, raucPath, raucDownloadPath string) (string, error) {
	bundleMetadata, err := service.getFileMetadata(ctx, raucPath)
	if err != nil {
		return "", err
	}

	raucStateDir := path.Join(service.cacheDirectory, "rauc")
	raucStateFile := path.Join(raucStateDir, "state.json")

	doDownload := false
	if _, err := os.Stat(raucStateFile); os.IsNotExist(err) {
		doDownload = true
	} else {
		stateBytes, err := os.ReadFile(raucStateFile)
		if err != nil {
			return "", err
		}

		var stateData FileMetadata
		if err := json.Unmarshal(stateBytes, &stateData); err != nil {
			return "", err
		}

		if stateData.SHA256() != bundleMetadata.SHA256() {
			doDownload = true
		}
	}

	if doDownload {

		if _, err := service.downloadMetadataCompare(ctx, "", raucPath, raucDownloadPath, bundleMetadata); err != nil {
			return "", err
		}

		stateBytes, err := json.Marshal(bundleMetadata)
		if err != nil {
			return "", err
		}

		if err := writeFileAnchored(raucStateFile, stateBytes, 0700, 0600); err != nil {
			return "", err
		}
	}

	// Check if the rauc bundle is available, if not return an empty string
	if _, err := os.Stat(raucDownloadPath); os.IsNotExist(err) {
		return "", nil
	}

	return raucDownloadPath, nil
}

// generateStreamingURL returns a signed URL the RAUC bundle can be streamed from, or an empty string if the
// config endpoint is currently unreachable.
func generateStreamingURL(service *Service, raucBundle string) (string, error) {
	// do not generate the streaming url if config endpoint is unavailable
	if service.IsConfigEndpointUnreachable() {
		return "", nil
	}

	filePath := path.Join(fileManagerPublicAPIPath, raucBundle)
	return service.urlSigner.SignURL(filePath)
}

// redact replaces the resolved local raucPath with the original configured bundle reference in log/error output,
// so logs don't leak internal download/temp paths.
func (r *RaucBundle) redact(s string, raucPath string) string {
	if r.Download {
		return s
	}
	return strings.ReplaceAll(s, raucPath, r.RaucBundle)
}
