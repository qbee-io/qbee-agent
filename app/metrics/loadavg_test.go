package metrics

import (
	"testing"
	"time"

	"qbee.io/platform/shared/test/assert"
)

func TestCollectLoadAverage(t *testing.T) {
	gotMetrics, err := CollectLoadAverage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.Length(t, gotMetrics, 1)

	metric := gotMetrics[0]

	assert.Equal(t, metric.Label, LoadAverage)
	assert.Empty(t, metric.ID)

	if time.Since(time.Unix(metric.Timestamp, 0)) > time.Second {
		t.Fatalf("invalid timestamp, got: %v", metric.Timestamp)
	}
}
