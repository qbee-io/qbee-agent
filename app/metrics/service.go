package metrics

import (
	"time"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/log"
)

const defaultAgentInterval = 5 // minutes

type Service struct {
	api                   *api.Client
	previousCPUValues     *CPUValues
	previousNetworkValues map[string]*NetworkValues
}

// New returns a new instance of metrics Service.
func New(apiClient *api.Client) *Service {
	return &Service{
		api:                   apiClient,
		previousNetworkValues: make(map[string]*NetworkValues),
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
		name: "memory",
		fn:   CollectMemory,
	},
	{
		name: "filesystem",
		fn:   CollectFilesystem,
	},
}

// Collect system metrics.
// If any errors are encountered, they'll be logged, but won't interrupt the process.
func (s *Service) Collect() []Metric {
	allMetrics := make([]Metric, 0)

	// collect metrics which don't depend on state
	for _, collector := range metricsCollectors {
		if metrics, err := collector.fn(); err != nil {
			log.Errorf("%s metrics error: %v", collector.name, err)
		} else {
			allMetrics = append(allMetrics, metrics...)
		}
	}

	// collect metrics which require previous state to produce delta-metrics
	if cpuMetric, err := s.doCollectCPU(); err != nil {
		log.Errorf("cpu metrics error: %v", err)
	} else if cpuMetric != nil {
		allMetrics = append(allMetrics, *cpuMetric)
	}

	if networkMetrics, err := s.doCollectNetwork(); err != nil {
		log.Errorf("network metrics error: %v", err)
	} else if networkMetrics != nil {
		allMetrics = append(allMetrics, networkMetrics...)
	}

	return allMetrics
}

func (s *Service) doCollectCPU() (*Metric, error) {

	cpuValues, err := CollectCPU()

	if err != nil {
		return nil, err
	}

	if s.previousCPUValues == nil {
		s.previousCPUValues = cpuValues
		log.Debugf("previous CPU values are nil")
		return nil, nil
	}

	cpuMetric, err := cpuValues.Delta(s.previousCPUValues)

	if err != nil {
		return nil, err
	}

	s.previousCPUValues = cpuValues

	return &Metric{
		Label:     CPU,
		Timestamp: time.Now().Unix(),
		Values: Values{
			CPUValues: cpuMetric,
		},
	}, nil
}

func (s *Service) doCollectNetwork() ([]Metric, error) {

	networkValues, err := CollectNetwork()

	if err != nil {
		return nil, err
	}

	if len(s.previousNetworkValues) == 0 {
		s.storePreviousNetworkValuesToMap(networkValues)
		log.Debugf("previous network values are nil")
		return nil, nil
	}

	metrics := make([]Metric, 0)

	for _, networkValue := range networkValues {
		previous, ok := s.previousNetworkValues[networkValue.ID]
		if !ok {
			continue
		}

		networkMetric, err := networkValue.Values.NetworkValues.Delta(previous)
		if err != nil {
			return nil, err
		}

		metrics = append(metrics, Metric{
			Label:     Network,
			Timestamp: time.Now().Unix(),
			Values: Values{
				NetworkValues: networkMetric,
			},
		})
	}

	s.storePreviousNetworkValuesToMap(networkValues)
	return metrics, nil
}

func (s *Service) storePreviousNetworkValuesToMap(metrics []Metric) {
	newMap := make(map[string]*NetworkValues)
	for _, metric := range metrics {
		newMap[metric.ID] = metric.Values.NetworkValues
	}
	s.previousNetworkValues = newMap
}
