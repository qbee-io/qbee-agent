package metrics

import (
	"fmt"
	"testing"
	"time"

	"qbee.io/platform/test/assert"
)

func TestCollectNetwork(t *testing.T) {
	gotMetrics, err := CollectNetwork()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gotMetrics) == 0 {
		t.Fatalf("expected at least one network interface, got 0")
	}

	for _, metric := range gotMetrics {
		assert.Equal(t, metric.Label, Network)
		if metric.ID == "" {
			t.Fatalf("expected metric ID to be a network interface name, got an empty string")
		}

		if time.Since(time.Unix(metric.Timestamp, 0)) > time.Second {
			t.Fatalf("invalid timestamp, got: %v", metric.Timestamp)
		}

		fmt.Printf("%s -> %#v\n", metric.ID, metric.Values.NetworkValues)
	}
}
