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
		return err
	}

	if !CheckPreCondition(ctx, r.PreCondition) {
		return nil
	}

	raucStatus, err := image.GetRaucStatus(ctx)
	if err != nil {
		ReportError(ctx, err, "Failed to get RAUC status")
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
			strings.ReplaceAll(err.Error(), raucPath, r.RaucBundle),
			"Failed to get RAUC bundle info for '%s'", r.RaucBundle,
		)
		return err
	}

	if raucStatus.Compatible != raucBundleInfo.Compatible {
		ReportError(ctx, err, "RAUC bundle '%s' is not compatible with the system '%s'", raucBundleInfo.Compatible, raucStatus.Compatible)
		return err
	}

	execInstall, err := shouldInstall(raucStatus, raucBundleInfo)

	if err != nil {
		ReportError(ctx, err, "Failed to install RAUC bundle")
		return err
	}

	if !execInstall {
		return nil
	}

	raucInstallCmd := []string{"rauc", "install", raucPath}
	output, err := utils.RunCommand(ctx, raucInstallCmd)

	if err != nil {
		ReportError(
			ctx,
			strings.ReplaceAll(err.Error(), raucPath, r.RaucBundle),
			"Failed to install RAUC bundle",
		)
		return err
	}

	ReportInfo(
		ctx,
		strings.ReplaceAll(string(output), raucPath, r.RaucBundle),
		"RAUC bundle successfully installed '%s'",
		r.RaucBundle,
	)

	service.RebootAfterRun(ctx)
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

func (r RaucBundle) getRaucBundleInfo(ctx context.Context, url string) (*RaucBundleInfo, error) {

	raucInfoCmd := []string{"rauc", "info", "--output-format", "json", url}
	raucInfoBytes, err := utils.RunCommand(ctx, raucInfoCmd)

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

func shouldInstall(localRaucInfo *image.RaucStatus, remoteBundleData *RaucBundleInfo) (bool, error) {
	if remoteBundleData.Compatible != localRaucInfo.Compatible {
		return false, fmt.Errorf("RAUC bundle '%s' is not compatible with the system '%s'", remoteBundleData.Compatible, localRaucInfo.Compatible)
	}

	_, currentSlotData, err := getCurrentSlot(localRaucInfo)
	if err != nil {
		return false, err
	}

	if remoteBundleData.Hash == currentSlotData.SlotStatus.Bundle.Hash {
		return false, nil
	}

	// image is compatible and not installed on currently running slot
	return true, nil
}

func getCurrentSlot(localRaucInfo *image.RaucStatus) (string, *image.SlotData, error) {
	for _, slot := range localRaucInfo.Slots {
		for slotName, slotData := range slot {
			if slotData.Bootname == localRaucInfo.Booted {
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
	bundleMetadata, err := service.getFileMetadataFromAPI(ctx, raucPath)
	if err != nil {
		return "", err
	}

	raucStateDir := path.Join(service.cacheDirectory, "rauc")

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

		if err := os.WriteFile(raucStateFile, stateBytes, 0600); err != nil {
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
