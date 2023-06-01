package metrics

import (
	"fmt"
	"math"
	"time"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/log"
)

const defaultAgentInterval = 5 // minutes

type Service struct {
	api         *api.Client
	prevSamples map[string][]Metric
}

// New returns a new instance of metrics Service.
func New(apiClient *api.Client) *Service {
	return &Service{
		api:         apiClient,
		prevSamples: make(map[string][]Metric),
	}
}

type metricsCollector struct {
	name          string
	fn            func() ([]Metric, error)
	deltaFunction func(previous, current []Metric) ([]Metric, error)
}

var metricsCollectors = []metricsCollector{
	{
		name: "load average",
		fn:   CollectLoadAverage,
	},
	{
		name:          "cpu",
		fn:            CollectCPU,
		deltaFunction: deltaCPU,
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
		name:          "network",
		fn:            CollectNetwork,
		deltaFunction: deltaNetwork,
	},
}

// Collect system metrics.
// If any errors are encountered, they'll be logged, but won't interrupt the process.
func (s *Service) Collect() []Metric {
	allMetrics := make([]Metric, 0)

	for _, collector := range metricsCollectors {
		if metrics, err := s.doCollect(collector); err != nil {
			log.Errorf("%s metrics error: %v", collector.name, err)
		} else {
			allMetrics = append(allMetrics, metrics...)
		}

	}

	return allMetrics
}

func (s *Service) doCollect(collector metricsCollector) ([]Metric, error) {
	if collector.deltaFunction != nil {
		return s.doCollectDelta(collector)
	}
	return collector.fn()
}

func (s *Service) doCollectDelta(collector metricsCollector) ([]Metric, error) {
	_, ok := s.prevSamples[collector.name]

	if !ok {
		// Missing previous sample, so we'll collect one now
		sample, err := collector.fn()
		if err != nil {
			return nil, err
		}
		s.prevSamples[collector.name] = sample
		// Sleep for one second to get a delta
		time.Sleep(1 * time.Second)
	}

	current, err := collector.fn()
	if err != nil {
		return nil, err
	}

	delta, err := collector.deltaFunction(s.prevSamples[collector.name], current)
	if err != nil {
		return nil, err
	}
	s.prevSamples[collector.name] = current

	return delta, nil
}

func sanityCheckValue(metric []Metric) error {
	if metric == nil {
		return fmt.Errorf("invalid metric values")
	}

	if len(metric) < 1 {
		return fmt.Errorf("invalid metric values")
	}

	return nil
}

func deltaCPU(previous, current []Metric) ([]Metric, error) {

	if err := sanityCheckValue(previous); err != nil {
		return nil, err
	}

	previousCPU := previous[0].Values.CPUValues
	if previousCPU == nil {
		return nil, fmt.Errorf("invalid previous CPU values")
	}

	if err := sanityCheckValue(current); err != nil {
		return nil, err
	}

	currentCPU := current[0].Values.CPUValues
	if currentCPU == nil {
		return nil, fmt.Errorf("invalid current CPU values")
	}

	currentCPUtotal := currentCPU.User + currentCPU.Nice + currentCPU.System + currentCPU.Idle + currentCPU.IOWait + currentCPU.IRQ
	previousCPUtotal := previousCPU.User + previousCPU.Nice + previousCPU.System + previousCPU.Idle + previousCPU.IOWait + previousCPU.IRQ

	elapsed := currentCPUtotal - previousCPUtotal

	if elapsed <= 0 {
		return nil, fmt.Errorf("invalid elapsed CPU values")
	}
	user := ((currentCPU.User - previousCPU.User) / elapsed) * 100
	nice := ((currentCPU.Nice - previousCPU.Nice) / elapsed) * 100
	system := ((currentCPU.System - previousCPU.System) / elapsed) * 100
	idle := ((currentCPU.Idle - previousCPU.Idle) / elapsed) * 100
	iowait := ((currentCPU.IOWait - previousCPU.IOWait) / elapsed) * 100
	irq := ((currentCPU.IRQ - previousCPU.IRQ) / elapsed) * 100
	// calculate delta

	deltaCPU := &CPUValues{
		User:   math.Round(user*100) / 100,
		Nice:   math.Round(nice*100) / 100,
		System: math.Round(system*100) / 100,
		Idle:   math.Round(idle*100) / 100,
		IOWait: math.Round(iowait*100) / 100,
		IRQ:    math.Round(irq*100) / 100,
	}

	return []Metric{
		{
			Label:     CPU,
			Timestamp: time.Now().Unix(),
			Values: Values{
				CPUValues: deltaCPU,
			},
		},
	}, nil
}

func deltaNetwork(previous, current []Metric) ([]Metric, error) {

	if err := sanityCheckValue(previous); err != nil {
		return nil, err
	}

	if err := sanityCheckValue(current); err != nil {
		return nil, err
	}

	metrics := make([]Metric, 0)

	for _, currentMetric := range current {

		if currentMetric.Values.NetworkValues == nil {
			return nil, fmt.Errorf("invalid current network values")
		}

		for _, previousMetric := range previous {
			if currentMetric.ID == previousMetric.ID {
				if previousMetric.Values.NetworkValues == nil {
					return nil, fmt.Errorf("invalid previous network values")
				}

				rxbytes := currentMetric.Values.NetworkValues.RXBytes - previousMetric.Values.NetworkValues.RXBytes
				if rxbytes < 0 {
					return nil, fmt.Errorf("invalid rxbytes")
				}
				txbytes := currentMetric.Values.NetworkValues.TXBytes - previousMetric.Values.NetworkValues.TXBytes
				if txbytes < 0 {
					return nil, fmt.Errorf("invalid rxbytes")
				}

				metric := Metric{
					Label:     Network,
					Timestamp: time.Now().Unix(),
					ID:        currentMetric.ID,
					Values: Values{
						NetworkValues: &NetworkValues{
							RXBytes: rxbytes,
							TXBytes: txbytes,
						},
					},
				}
				metrics = append(metrics, metric)
			}
		}
	}

	if len(metrics) == 0 {
		return nil, fmt.Errorf("invalid network values")
	}
	return metrics, nil
}
