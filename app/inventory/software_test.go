//go:build linux

package inventory_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/qbee-io/qbee-agent/app/inventory"
)

func TestCollectSoftwareInventory_Deb(t *testing.T) {
	softwareInventory, err := inventory.CollectSoftwareInventory(context.Background())
	if err != nil {
		t.Fatalf("error collecting software inventory: %v", err)
	}

	data, _ := json.MarshalIndent(softwareInventory, " ", " ")

	fmt.Println(string(data))
}
