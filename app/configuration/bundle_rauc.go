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
	"strings"

	"go.qbee.io/agent/app/image"
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

	// Must happen before any early return, or a record from another slot could match again after a fallback.
	// Eg. a manual fallback to a previous boot slot could leave a stale last-install record that matches the new boot slot.
	// which would cause the shouldInstall check to incorrectly assume the bundle was already installed.
	raucInstallStatusFile := path.Join(raucStateDirectory(service), raucInstallStatusFile)
	if err := discardStaleInstallStatusFile(raucInstallStatusFile, raucStatus.BootPrimary); err != nil {
		ReportError(ctx, err, "Failed to discard stale RAUC last-install record")
		return err
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

	execInstall, err := shouldInstall(raucStatus, raucBundleInfo, service)

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

	// Evaluate if a reboot is required after RAUC bundle installation
	if CheckCondition(ctx, r.RebootCondition) {
		service.RebootAfterRun(ctx)
	}

	// Some bootloader interfaces do not report the new slot as boot primary after install.
	// This ensures that the last installation record is updated correctly even if the boot primary slot did not change.
	if err := updateRaucInstallStatus(ctx, service, raucStatus, raucBundleInfo); err != nil {
		ReportError(ctx, err, "Failed to update last RAUC bundle installation record")
		return err
	}

	return nil
}

// updateRaucInstallStatus records the installed bundle if boot primary did not change after installation,
// otherwise it removes any previous record.
func updateRaucInstallStatus(
	ctx context.Context,
	service *Service,
	oldStatus *image.RaucStatus,
	bundleInfo *RaucBundleInfo,
) error {
	newStatus, err := image.GetRaucStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get new RAUC status after installation: %w", err)
	}

	stateDir := raucStateDirectory(service)
	raucInstallStatusFile := path.Join(stateDir, raucInstallStatusFile)

	if newStatus.BootPrimary == oldStatus.BootPrimary {
		if err := writeInstallStatusFile(stateDir, raucInstallStatusFile, newStatus.BootPrimary, bundleInfo); err != nil {
			return fmt.Errorf("failed to record last RAUC bundle installation: %w", err)
		}
		return nil
	}

	return removeInstallStatusFile(raucInstallStatusFile)
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

// raucInstallStatusFile records the last bundle installed while boot primary remained unchanged.
const raucInstallStatusFile = "rauc_install_status.json"

// raucStateDirectory returns the directory where RAUC bundle state is persisted.
func raucStateDirectory(service *Service) string {
	return path.Join(service.cacheDirectory, "rauc")
}

// raucInstallStatus is the persisted record of the last bundle installed while boot primary remained unchanged.
type raucInstallStatus struct {
	// BootPrimary is the boot primary slot at the time of installation.
	BootPrimary string `json:"boot_primary"`

	// InstalledBundleInfo is the installed bundle info.
	InstalledBundleInfo RaucBundleInfo `json:"bundle"`
}

func writeInstallStatusFile(stateDir, raucInstallStatusFile, bootPrimary string, bundleInfo *RaucBundleInfo) error {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return err
	}

	data, err := json.Marshal(raucInstallStatus{BootPrimary: bootPrimary, InstalledBundleInfo: *bundleInfo})
	if err != nil {
		return err
	}

	return os.WriteFile(raucInstallStatusFile, data, 0600)
}

func removeInstallStatusFile(raucInstallStatusFile string) error {
	if err := os.Remove(raucInstallStatusFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove RAUC last-install record: %w", err)
	}
	return nil
}

// readInstallStatusFile returns the last-install record, or nil if it is missing or corrupt.
func readInstallStatusFile(raucInstallStatusFile string) (*raucInstallStatus, error) {
	data, err := os.ReadFile(raucInstallStatusFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var raucLastInstallStatus raucInstallStatus
	if err := json.Unmarshal(data, &raucLastInstallStatus); err != nil {
		return nil, nil
	}

	return &raucLastInstallStatus, nil
}

// discardStaleInstallStatusFile removes the last-install record if it is corrupt or was written for another boot primary slot.
func discardStaleInstallStatusFile(raucInstallStatusFile, bootPrimary string) error {
	raucLastInstallStatus, err := readInstallStatusFile(raucInstallStatusFile)
	if err != nil {
		return err
	}

	// we only keep the last-install record if it matches the current boot primary slot, eg.
	// that the system has not switched to another boot primary slot since the last install.
	if raucLastInstallStatus != nil &&
		raucLastInstallStatus.InstalledBundleInfo.Hash != "" &&
		raucLastInstallStatus.BootPrimary == bootPrimary {
		return nil
	}

	return removeInstallStatusFile(raucInstallStatusFile)
}

// shouldInstall determines whether the RAUC bundle should be installed based on the current system state and the last install record (if available).
func shouldInstall(
	localRaucInfo *image.RaucStatus,
	remoteBundleData *RaucBundleInfo,
	service *Service,
) (bool, error) {

	if remoteBundleData.Compatible != localRaucInfo.Compatible {
		return false, fmt.Errorf("RAUC bundle '%s' is not compatible with the system '%s'", remoteBundleData.Compatible, localRaucInfo.Compatible)
	}

	_, currentSlotData, err := getPrimarySlot(localRaucInfo)
	if err != nil {
		return false, err
	}

	// Bundle is already running from the boot primary slot.
	if remoteBundleData.Hash == currentSlotData.SlotStatus.Bundle.Hash {
		return false, nil
	}

	raucInstallStatus, err := readInstallStatusFile(path.Join(raucStateDirectory(service), raucInstallStatusFile))
	if err != nil {
		return false, err
	}

	// Bundle was already installed, but the bootloader does not report it as the current boot primary yet
	if raucInstallStatus != nil && raucInstallStatus.InstalledBundleInfo.Hash == remoteBundleData.Hash {
		return false, nil
	}

	return true, nil
}

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

func downloadRaucBundle(ctx context.Context, service *Service, raucPath, raucDownloadPath string) (string, error) {
	bundleMetadata, err := service.getFileMetadata(ctx, raucPath)
	if err != nil {
		return "", err
	}

	raucStateDir := raucStateDirectory(service)

	if _, err := os.Stat(raucStateDir); os.IsNotExist(err) {
		if err := os.MkdirAll(raucStateDir, 0700); err != nil {
			return "", err
		}
	}

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

func generateStreamingURL(service *Service, raucBundle string) (string, error) {
	// do not generate the streaming url if config endpoint is unavailable
	if service.IsConfigEndpointUnreachable() {
		return "", nil
	}

	filePath := path.Join(fileManagerPublicAPIPath, raucBundle)
	return service.urlSigner.SignURL(filePath)
}

func (r *RaucBundle) redact(s string, raucPath string) string {
	if r.Download {
		return s
	}
	return strings.ReplaceAll(s, raucPath, r.RaucBundle)
}
