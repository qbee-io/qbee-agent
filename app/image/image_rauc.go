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

package image

import (
	"context"
	"encoding/json"
	"os/exec"

	"go.qbee.io/agent/app/utils"
)

// SlotData contains metadata of the RAUC slots
type SlotData struct {
	// Class - RAUC slot class
	Class string `json:"class"`

	// Device - RAUC slot device
	Device string `json:"device"`

	// Type - RAUC slot type (ext4, btrfs, ...)
	Type string `json:"type"`

	// Bootname - RAUC slot bootname (A, B, ...)
	Bootname string `json:"bootname"`

	// State - RAUC slot state (booted, inactive ...)
	State string `json:"state"`

	// Parent - RAUC slot parent
	Parent string `json:"parent"`

	// Mountpoint - RAUC slot mountpoint
	Mountpoint string `json:"mountpoint"`

	// BootStatus - RAUC slot boot status (good, bad, ...)
	BootStatus string `json:"boot_status"`

	// SlotStatus - RAUC slot status
	SlotStatus struct {

		// Bundle - RAUC slot bundle metadata
		Bundle struct {
			// Compatible - RAUC compatible
			Compatible string `json:"compatible"`

			// Version - RAUC version
			Version string `json:"version"`

			// Description - RAUC bundle description
			Description string `json:"description"`

			// Build - RAUC bundle build id/number
			Build string `json:"build"`

			// Hash - RAUC bundle hash
			Hash string `json:"hash"`
		} `json:"bundle"`
		// Checksum - RAUC slot checksum metadata
		Checksum struct {
			// Sha256 - RAUC slot checksum SHA256
			Sha256 string `json:"sha256"`

			// Size - RAUC slot size
			Size int `json:"size"`
		} `json:"checksum"`

		// Installed - RAUC slot installation metadata
		Installed struct {
			// Timestamp - RAUC slot installation timestamp
			Timestamp string `json:"timestamp"`

			// Count - RAUC slot installation count
			Count int `json:"count"`
		} `json:"installed"`

		// Activated - RAUC slot activation metadata
		Activated struct {

			// Timestamp - RAUC slot activation timestamp
			Timestamp string `json:"timestamp"`

			// Count - RAUC slot activation count
			Count int `json:"count"`
		} `json:"activated"`

		// Status - RAUC slot status
		Status string `json:"status"`
	} `json:"slot_status"`
}

// RaucStatus contains the current RAUC status of the system
type RaucStatus struct {
	// Compatible - RAUC compatible version.
	Compatible string `json:"compatible"`

	// Variant - RAUC variant.
	Variant string `json:"variant"`

	// Booted - currently booted partition (A or B)
	Booted string `json:"booted"`

	// BootPrimary - primary boot slot for next boot
	BootPrimary string `json:"boot_primary"`

	// Slots - RAUC slots
	Slots []map[string]SlotData `json:"slots"`
}

// HasRauc returns true if RAUC is installed on the system.
func HasRauc() bool {
	_, err := exec.LookPath("rauc")
	return err == nil
}

// GetRaucInfo returns RAUC information.
func GetRaucInfo(ctx context.Context) (*RaucStatus, error) {

	raucStatusCmd := []string{"rauc", "status", "--output-format", "json", "--detailed"}

	raucInfoBytes, err := utils.RunCommand(ctx, raucStatusCmd)

	if err != nil {
		return nil, err
	}

	var raucInfo RaucStatus
	err = json.Unmarshal(raucInfoBytes, &raucInfo)
	if err != nil {
		return nil, err
	}
	return &raucInfo, nil
}
