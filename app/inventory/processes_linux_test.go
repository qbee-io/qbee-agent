package inventory

import (
	"testing"

	"qbee.io/platform/test/assert"
)

func TestCollectProcessesInventory(t *testing.T) {
	processes, err := CollectProcessesInventory()
	if err != nil {
		t.Fatalf("error collecting processes: %v", err)
	}

	if len(processes.Processes) == 0 {
		t.Fatalf("no processes collected")
	}

	for _, process := range processes.Processes {
		assert.NotEmpty(t, process.PID)
		assert.NotEmpty(t, process.User)
		assert.NotEmpty(t, process.Command)
	}
}
