package image

import (
	"context"
	"encoding/json"
	"os/exec"

	"go.qbee.io/agent/app/utils"
)

type SlotData struct {
	Class      string `json:"class"`
	Device     string `json:"device"`
	Type       string `json:"type"`
	Bootname   string `json:"bootname"`
	State      string `json:"state"`
	Parent     string `json:"parent"`
	Mountpoint string `json:"mountpoint"`
	BootStatus string `json:"boot_status"`
	SlotStatus struct {
		Bundle struct {
			Compatible  string `json:"compatible"`
			Version     string `json:"version"`
			Description string `json:"description"`
			Build       string `json:"build"`
			Hash        string `json:"hash"`
		} `json:"bundle"`
		Checksum struct {
			Sha256 string `json:"sha256"`
			Size   int    `json:"size"`
		} `json:"checksum"`
		Installed struct {
			Timestamp string `json:"timestamp"`
			Count     int    `json:"count"`
		} `json:"installed"`
		Activated struct {
			Timestamp string `json:"timestamp"`
			Count     int    `json:"count"`
		} `json:"activated"`
		Status string `json:"status"`
	} `json:"slot_status"`
}

type RaucInfo struct {
	Compatible  string                `json:"compatible"`
	Variant     string                `json:"variant"`
	Booted      string                `json:"booted"`
	BootPrimary string                `json:"boot_primary"`
	Slots       []map[string]SlotData `json:"slots"`
}

func HasRauc() bool {
	_, err := exec.LookPath("rauc")
	if err != nil {
		return false
	}
	return true
}

func GetRaucInfo(ctx context.Context) (*RaucInfo, error) {

	raucStatusCmd := []string{"rauc", "status", "--output-format", "json", "--detailed"}

	raucInfoBytes, err := utils.RunCommand(ctx, raucStatusCmd)

	if err != nil {
		return nil, err
	}

	var raucInfo RaucInfo
	err = json.Unmarshal(raucInfoBytes, &raucInfo)
	if err != nil {
		return nil, err
	}
	return &raucInfo, nil
}
