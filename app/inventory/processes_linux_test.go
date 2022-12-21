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

	for _, process := range processes.Processes {
		if process.PID != 20543 {
			continue
		}
		data, _ := json.MarshalIndent(process, " ", " ")

		fmt.Println(string(data))
	}
}
