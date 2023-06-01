package metrics

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/qbee-io/qbee-agent/app/api"
)

func TestCollectServiceCollectAll(t *testing.T) {
	apiClient, _ := api.NewMockedClient()

	srv := New(apiClient)

	gotMetrics := srv.Collect()

	if len(gotMetrics) == 0 {
		t.Fatalf("expected at least one metric, got 0")
	}

	metricBytes, err := json.MarshalIndent(gotMetrics, "", "  ")

	if err != nil {
		t.Fatalf("error marshaling metrics: %v", err)
	}

	fmt.Printf("got metrics: %s", string(metricBytes))
}
