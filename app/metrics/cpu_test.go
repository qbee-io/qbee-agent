package metrics

import (
	"testing"
	"time"
)

func TestCollectCPU(t *testing.T) {
	v, err := CollectCPU()
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}

	time.Sleep(1 * time.Second)

	v2, err := CollectCPU()
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}

	v3, err := v2.Delta(v)
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}
	// verify that sum of values is about 100% (accounted for rounding errors)

	total := v3.User + v3.Nice + v3.System + v3.Idle + v3.IOWait + v3.IRQ
	d := 100 - total
	if d > 1 || d < -1 {
		t.Fatalf("expected total of 100%%, got %f%%", total)
	}
}
