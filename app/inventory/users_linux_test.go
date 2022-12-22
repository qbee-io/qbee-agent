//go:build linux

package inventory_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/qbee-io/qbee-agent/app/inventory"
)

func TestCollectUsersInventory(t *testing.T) {
	users, err := inventory.CollectUsersInventory()
	if err != nil {
		t.Fatalf("error collecting users inventory: %v", err)
	}

	data, _ := json.MarshalIndent(users, " ", " ")

	fmt.Println(string(data))
}
