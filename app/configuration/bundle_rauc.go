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
	"os/exec"
	"path"
	"strings"

	"go.qbee.io/agent/app/image"
	"go.qbee.io/agent/app/utils"
)

// example payload
// {
//   "pre_condition": "true",
//   "rauc_bundle": "/path/to/bundle.rauc",
// }

// RaucBundle configures the system to install a RAUC bundle.
type RaucBundle struct {
	Metadata

	// PreCondition is a condition that must be met before the bundle is executed.
	PreCondition string `json:"pre_condition"`

	// RaucBundle is the path to the RAUC bundle file.
	RaucBundle string `json:"rauc_bundle"`
}

// Execute RAUC bundle configuration on the system.
func (r RaucBundle) Execute(ctx context.Context, service *Service) error {
	// check if the pre-condition is met

	if !image.HasRauc() {
		ReportError(ctx, nil, "RAUC not found")
		return nil
	}

	if !CheckPreCondition(ctx, r.PreCondition) {
		return nil
	}

	raucStatus, err := image.GetRaucInfo(ctx)
	if err != nil {
		ReportError(ctx, err, "Failed to get RAUC status")
		return err
	}

	filePath := path.Join(fileManagerPublicAPIPath, r.RaucBundle)
	raucUrl, err := service.urlSigner.SignURL(filePath)
	if err != nil {
		ReportError(ctx, err, "Failed to create authenticated url for rauc")
		return err
	}

	raucBundleInfo, err := r.getRaucBundleInfo(ctx, raucUrl)
	if err != nil {
		ReportError(
			ctx,
			strings.ReplaceAll(err.Error(), raucUrl, redactedValue),
			"Failed to get RAUC bundle info for '%s'", r.RaucBundle,
		)
		return err
	}

	if raucStatus.Compatible != raucBundleInfo.Compatible {
		ReportError(ctx, err, "RAUC bundle '%s' is not compatible with the system '%s'", raucBundleInfo.Compatible, raucStatus.Compatible)
		return err
	}

	execInstall, err := doInstall(raucStatus, raucBundleInfo)

	if err != nil {
		ReportError(ctx, err, "Failed to install RAUC bundle")
		return err
	}

	if !execInstall {
		return nil
	}

	raucCmd, err := exec.LookPath("rauc")
	if err != nil {
		ReportError(ctx, err, "RAUC not found")
		return err
	}

	raucInstallCmd := []string{raucCmd, "install", raucUrl}
	output, err := utils.RunCommand(ctx, raucInstallCmd)

	if err != nil {
		ReportError(
			ctx,
			strings.ReplaceAll(err.Error(), raucUrl, redactedValue),
			"Failed to install RAUC bundle",
		)
		return err
	}

	ReportInfo(
		ctx,
		strings.ReplaceAll(string(output), raucUrl, redactedValue),
		"RAUC bundle successfully installed '%s'",
		r.RaucBundle,
	)

	service.RebootAfterRun(ctx)
	return nil
}

type RaucImageInfo struct {
	Label    string `json:"label,omitempty"`
	Variant  string `json:"variant"`
	Filename string `json:"filename"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`

	Adaptive []string `json:"adaptive"`
}

type RaucBundleInfo struct {
	Compatible  string                     `json:"compatible"`
	Version     string                     `json:"version"`
	Description string                     `json:"description"`
	Build       string                     `json:"build"`
	Hash        string                     `json:"hash"`
	Images      []map[string]RaucImageInfo `json:"images"`
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

func doInstall(localRaucInfo *image.RaucInfo, remoteBundleData *RaucBundleInfo) (bool, error) {
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

func getCurrentSlot(localRaucInfo *image.RaucInfo) (string, *image.SlotData, error) {
	for _, slot := range localRaucInfo.Slots {
		for slotName, slotData := range slot {
			if slotData.Bootname == localRaucInfo.Booted {
				return slotName, &slotData, nil
			}
		}
	}
	return "", nil, fmt.Errorf("No slot found in RAUC info")
}