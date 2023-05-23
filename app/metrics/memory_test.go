package metrics

import (
	"testing"
	"time"

	"qbee.io/platform/test/assert"
)

func TestCollectMemory(t *testing.T) {
	gotMetrics, err := CollectMemory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.Length(t, gotMetrics, 1)

	metric := gotMetrics[0]

	assert.Equal(t, metric.Label, Memory)
	assert.Empty(t, metric.ID)

	if time.Since(time.Unix(metric.Timestamp, 0)) > time.Second {
		t.Fatalf("invalid timestamp, got: %v", metric.Timestamp)
	}

	//fmt.Printf("%#v\n", metric.Values.MemoryValues)
}
