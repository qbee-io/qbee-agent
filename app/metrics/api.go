package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

const metricsBatchSize = 100

// Send delivers metrics to the device hub.
func (srv *Service) Send(ctx context.Context, metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	srv.addMetricsToBuffer(metrics)

	path := fmt.Sprintf("/v1/org/device/auth/metric")

	srv.bufferLock.Lock()
	defer srv.bufferLock.Unlock()

	// send metrics in batches until the buffer is empty or API error occurs
	for len(srv.buffer) > 0 {
		buf := new(bytes.Buffer)
		encoder := json.NewEncoder(buf)
		count := 0

		for _, metric := range srv.buffer {
			if err := encoder.Encode(metric); err != nil {
				return fmt.Errorf("error encoding metrics payload: %w", err)
			}

			if count++; count >= metricsBatchSize {
				break
			}
		}

		if err := srv.api.Post(ctx, path, buf, nil); err != nil {
			return fmt.Errorf("error sending metrics request: %w", err)
		}

		// remove delivered metrics from the buffer
		srv.buffer = srv.buffer[count:]
	}

	return nil
}
