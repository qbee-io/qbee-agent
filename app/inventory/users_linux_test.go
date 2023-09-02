//go:build linux

package inventory_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/qbee-io/qbee-agent/app/inventory"
)

func TestCollectUsersInventory(t *testing.T) {
	if _, err := os.ReadFile(inventory.ShadowFilePath); err != nil {
		t.Skipf("can't open /etc/shadow - skipping test: %v", err)
	}

	users, err := inventory.CollectUsersInventory()
	if err != nil {
		t.Fatalf("error collecting users inventory: %v", err)
	}

	data, _ := json.MarshalIndent(users, " ", " ")

	fmt.Println(string(data))
}
