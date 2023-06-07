package metrics

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/qbee-io/qbee-agent/app/inventory/linux"
	"github.com/qbee-io/qbee-agent/app/utils"
)

// NetworkValues
//
// Example payload:
//
//	{
//	 "label": "network",
//	 "ts": 1669988326,
//	 "id": "eth0",
//	 "values": {
//	   "tx_bytes": 7126,
//	   "rx_bytes": 17423
//	 }
//	}
type NetworkValues struct {
	// Received bytes on a network interface
	RXBytes int `json:"rx_bytes"`
	// Transferred bytes on a network interface
	TXBytes int `json:"tx_bytes"`
}

// CollectNetwork metrics.
// Note: collected are total values. The agent must report delta,
// so we need to keep state from the last report and subtract it before delivery.
func CollectNetwork() ([]Metric, error) {
	path := filepath.Join(linux.ProcFS, "net", "dev")

	metrics := make([]Metric, 0)

	err := utils.ForLinesInFile(path, func(line string) error {
		fields := strings.Fields(line)

		if !strings.HasSuffix(fields[0], ":") {
			return nil
		}

		ifaceName := strings.TrimSuffix(fields[0], ":")

		rxBytes, err := strconv.Atoi(fields[1])
		if err != nil {
			return err
		}

		var txBytes int
		if txBytes, err = strconv.Atoi(fields[9]); err != nil {
			return err
		}

		metric := Metric{
			Label:     Network,
			Timestamp: time.Now().Unix(),
			ID:        ifaceName,
			Values: Values{
				NetworkValues: &NetworkValues{
					RXBytes: rxBytes,
					TXBytes: txBytes,
				},
			},
		}

		metrics = append(metrics, metric)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

func (v *NetworkValues) Delta(old *NetworkValues) (*NetworkValues, error) {
	if old == nil {
		return v, nil
	}

	return &NetworkValues{
		RXBytes: v.RXBytes - old.RXBytes,
		TXBytes: v.TXBytes - old.TXBytes,
	}, nil
}
