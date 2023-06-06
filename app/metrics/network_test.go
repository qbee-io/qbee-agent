package metrics

import (
	"testing"
	"time"

	"qbee.io/platform/test/assert"
)

func TestCollectNetwork(t *testing.T) {
	v1, err := CollectNetwork()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(v1) == 0 {
		t.Fatalf("expected at least one network interface, got 0")
	}

	time.Sleep(1 * time.Second)

	v2, err := CollectNetwork()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, metric := range v2 {
		for _, previous := range v1 {

			if metric.ID != previous.ID {
				continue
			}
			v3, err := metric.Values.NetworkValues.Delta(previous.Values.NetworkValues)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			t.Logf("%s -> %#v\n", metric.ID, v3)

			assert.Equal(t, metric.Label, Network)
		}
		if metric.ID == "" {
			t.Fatalf("expected metric ID to be a network interface name, got an empty string")
		}

		if time.Since(time.Unix(metric.Timestamp, 0)) > time.Second {
			t.Fatalf("invalid timestamp, got: %v", metric.Timestamp)
		}

	}
}
