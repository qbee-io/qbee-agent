package metrics

import (
	"context"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/log"
)

const defaultAgentInterval = 5 // minutes

type Service struct {
	api *api.Client
}

// New returns a new instance of metrics Service.
func New(apiClient *api.Client) *Service {
	return &Service{
		api: apiClient,
	}
}

type metricsCollector struct {
	name string
	fn   func() ([]Metric, error)
}

var metricsCollectors = []metricsCollector{
	{
		name: "load average",
		fn:   CollectLoadAverage,
	},
	{
		name: "cpu",
		fn:   CollectCPU,
	},
	{
		name: "memory",
		fn:   CollectMemory,
	},
	{
		name: "filesystem",
		fn:   CollectFilesystem,
	},
	{
		name: "network",
		fn:   CollectNetwork,
	},
}

// Collect system metrics.
// If any errors are encountered, they'll be logged, but won't interrupt the process.
func Collect(ctx context.Context) []Metric {
	allMetrics := make([]Metric, 0)

	for _, collector := range metricsCollectors {
		if metrics, err := collector.fn(); err != nil {
			log.Errorf("%s metrics error: %v", collector.name, err)
		} else {
			allMetrics = append(allMetrics, metrics...)
		}

	}

	return allMetrics
}
