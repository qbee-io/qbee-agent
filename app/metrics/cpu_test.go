package metrics

import (
	"testing"
	"time"

	"qbee.io/platform/shared/test/assert"
)

func TestCollectCPU(t *testing.T) {
	got, err := CollectCPU()
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}

	assert.Length(t, got, 1)

	metric := got[0]

	assert.Equal(t, metric.Label, CPU)
	assert.Empty(t, metric.ID)

	if time.Since(time.Unix(metric.Timestamp, 0)) > time.Second {
		t.Fatalf("invalid timestamp, got: %v", metric.Timestamp)
	}

	// verify that sum of values is about 100% (accounted for rounding errors)
	v := metric.Values.CPUValues

	total := v.User + v.Nice + v.System + v.Idle + v.IOWait + v.IRQ
	d := 100 - total
	if d > 1 || d < -1 {
		t.Fatalf("expected total of 100%%, got %f%%", total)
	}
}
