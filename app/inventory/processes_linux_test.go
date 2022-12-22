package inventory

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCollectProcessesInventory(t *testing.T) {
	processes, err := CollectProcessesInventory()
	if err != nil {
		t.Fatalf("error collecting processes: %v", err)
	}

	data, _ := json.MarshalIndent(processes, " ", " ")

	fmt.Println(string(data))
}
