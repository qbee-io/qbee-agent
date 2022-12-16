package inventory

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCollectPortsInventory(t *testing.T) {
	ports, err := CollectPortsInventory()
	if err != nil {
		t.Fatalf("error collecting ports: %v", err)
	}

	data, _ := json.MarshalIndent(ports, " ", " ")

	fmt.Println(string(data))
}
