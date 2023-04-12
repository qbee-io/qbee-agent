package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// Send delivers metrics to the device hub.
func (srv *Service) Send(ctx context.Context, metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	path := fmt.Sprintf("/v1/org/device/auth/metric")

	buf := new(bytes.Buffer)

	encoder := json.NewEncoder(buf)

	for _, metric := range metrics {
		if err := encoder.Encode(metric); err != nil {
			return fmt.Errorf("error encoding metrics payload: %w", err)
		}
	}

	if err := srv.api.Post(ctx, path, buf, nil); err != nil {
		return fmt.Errorf("error sending metrics request: %w", err)
	}

	return nil
}
