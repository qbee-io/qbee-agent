package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/qbee-io/qbee-agent/app/test"
)

func TestCollectFilesystem(t *testing.T) {
	gotMetrics, err := CollectFilesystem()
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}

	if len(gotMetrics) == 0 {
		t.Fatalf("expected at least one filesystem, got 0")
	}

	for _, metric := range gotMetrics {
		test.Equal(t, metric.Label, Filesystem)

		if !strings.HasPrefix(metric.ID, "/") {
			t.Fatalf("expected filesystem mount to be under root, got %s", metric.ID)
		}

		if time.Since(time.Unix(metric.Timestamp, 0)) > time.Second {
			t.Fatalf("invalid timestamp, got: %v", metric.Timestamp)
		}
	}
}
