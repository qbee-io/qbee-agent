//go:build linux

package inventory_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/qbee-io/qbee-agent/app/inventory"
	"github.com/qbee-io/qbee-agent/app/inventory/software"
)

func TestCollectSoftwareInventory_Deb(t *testing.T) {
	softwareInventory, err := inventory.CollectSoftwareInventory(software.DebPackageManagerType)
	if err != nil {
		t.Fatalf("error collecting software inventory: %v", err)
	}

	data, _ := json.MarshalIndent(softwareInventory, " ", " ")

	fmt.Println(string(data))
}
