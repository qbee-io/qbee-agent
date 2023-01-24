package metrics

import (
	"testing"
	"time"

	"github.com/qbee-io/qbee-agent/app/test"
)

func TestCollectMemory(t *testing.T) {
	gotMetrics, err := CollectMemory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	test.Length(t, gotMetrics, 1)

	metric := gotMetrics[0]

	test.Equal(t, metric.Label, Memory)
	test.Empty(t, metric.ID)

	if time.Since(time.Unix(metric.Timestamp, 0)) > time.Second {
		t.Fatalf("invalid timestamp, got: %v", metric.Timestamp)
	}

	//fmt.Printf("%#v\n", metric.Values.MemoryValues)
}
