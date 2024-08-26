// Copyright 2023 qbee.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"sync"
	"time"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/utils/cache"
)

// Service collects system metrics and sends them to the device hub.
type Service struct {
	api                   *api.Client
	previousCPUValues     *CPUValues
	previousNetworkValues map[string]*NetworkValues
	lock                  sync.Mutex
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
	{
		name: "temperature",
		fn:   CollectTemperature,
	},
}

const metricsCacheKey = "metrics"

var metricsCacheTTL = 60 * time.Second

// Collect system metrics.
// If any errors are encountered, they'll be logged, but won't interrupt the process.
func (s *Service) Collect() []Metric {
	s.lock.Lock()
	defer s.lock.Unlock()

	if cachedMetrics, ok := cache.Get(metricsCacheKey); ok {
		return cachedMetrics.([]Metric)
	}

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

	cache.Set(metricsCacheKey, allMetrics, metricsCacheTTL)

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
			ID:        networkValue.ID,
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
