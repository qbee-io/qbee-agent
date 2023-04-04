package metrics

import (
	"sync"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/log"
)

// 10 days of 12 metrics per hour for 8 recorded metrics
const metricsBufferSize = 10 * 24 * 12 * 8

type Service struct {
	api        *api.Client
	buffer     []Metric
	bufferLock sync.Mutex
}

// New returns a new instance of metrics Service.
func New(apiClient *api.Client) *Service {
	return &Service{
		api:    apiClient,
		buffer: make([]Metric, 0),
	}
}

// addMetricsToBuffer adds metrics to the buffer, dropping the oldest ones if the buffer is full.
func (srv *Service) addMetricsToBuffer(metrics []Metric) {
	srv.bufferLock.Lock()
	defer srv.bufferLock.Unlock()

	if len(srv.buffer)+len(metrics) > metricsBufferSize {
		log.Warnf("metrics buffer is full, dropping %d metrics", len(metrics))
		srv.buffer = append(srv.buffer[len(metrics):], metrics...)
		return
	}

	srv.buffer = append(srv.buffer, metrics...)
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
func Collect() []Metric {
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
