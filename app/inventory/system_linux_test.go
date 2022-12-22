//go:build linux

package inventory_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/qbee-io/qbee-agent/app/inventory"
)

func TestCollectSystemInventory(t *testing.T) {
	systemInfo, err := inventory.CollectSystemInventory()
	if err != nil {
		t.Fatalf("error collecting system info: %v", err)
	}

	data, _ := json.MarshalIndent(systemInfo, " ", " ")

	fmt.Println(string(data))
}
