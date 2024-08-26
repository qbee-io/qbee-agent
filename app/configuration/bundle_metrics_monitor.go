// Copyright 2024 qbee.io
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

// Metrics []MetricMonitor `json:"metrics" bson:"metrics"`
// MetricsMonitor configures on-agent metrics monitoring.
//
// Example payload:
//
//	{
//	  "metrics": [
//	 	{
//	 		"value": "cpu:user",
//			"threshold": 20.0
//	 	}
//	  ]
//	}

package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go.qbee.io/agent/app/metrics"
)

// MetricsMonitorBundle configures on-agent metrics monitoring.
//
// Example payload:
//
//	{
//	  "metrics": [
//	 	{
//	 		"value": "cpu:user",
//			"threshold": 20.0
//	 	},
//		{
//	 		"value": "filesystem:use",
//			"threshold": 60.0,
//			"id": "/data"
//	 	},
//
//	  ]
//	}

type MetricsMonitorBundle struct {
	Metadata

	// Metrics to monitor.
	Metrics []MetricMonitor `json:"metrics" bson:"metrics"`
}

// MetricMonitor defines monitor for a single metric.
type MetricMonitor struct {

	// Value of the metric to monitor.
	Value string `json:"value"`

	// Threshold above which a warning will be created by the device.
	Threshold float64 `json:"threshold"`

	// ID of the resource (e.g. filesystem mount point)
	ID string `json:"id,omitempty"`
}

var metricsMonitorStatelock sync.Mutex
var metricsMonitorState map[string]float64

// initialize the metrics monitor state
func init() {
	metricsMonitorState = make(map[string]float64)
}

// Execute the metrics monitor bundle.
func (m *MetricsMonitorBundle) Execute(ctx context.Context, service *Service) error {

	collectedMetrics := service.metrics.Collect()

	reports, err := m.EvaluateMonitors(collectedMetrics)
	if err != nil {
		return err
	}

	if len(reports) < 1 {
		return nil
	}

	for _, report := range reports {
		addReport(ctx, report.Severity, nil, report.Text)
	}
	return nil
}

// EvaluateMonitors evaluates the metrics monitors and returns a list of reports
func (m *MetricsMonitorBundle) EvaluateMonitors(values []metrics.Metric) ([]Report, error) {
	metricValues, err := metricsToMap(values)
	metricsMonitorsMap := metricMonitorsToMap(m.Metrics)

	if err != nil {
		return nil, err
	}

	if len(metricsMonitorsMap) == 0 {
		return nil, nil
	}

	reports := make([]Report, 0)

	// Clean up the state for monitors that are not defined anymore.
	tidyMonitorState(metricsMonitorsMap)

	for monitor, monitorValue := range metricsMonitorsMap {
		report := m.evaluateMonitor(metricValues, monitor, monitorValue)

		if report != nil {
			reports = append(reports, *report)
		}
	}
	return reports, nil
}

// evaluateMonitor evaluates a single monitor and returns a report if the monitor has triggered or recovered
func (m *MetricsMonitorBundle) evaluateMonitor(metricValues map[string]float64, monitor string, monitorValue float64) *Report {
	if value, ok := metricValues[monitor]; ok {
		if value >= monitorValue && !getMonitorState(monitor) {
			setMonitorState(monitor, monitorValue)
			return &Report{
				Severity: severityWarning,
				Text:     fmt.Sprintf("Metrics monitor %s triggered, value %.2f >= %.2f", monitor, value, monitorValue),
			}
		}
		if value < monitorValue && getMonitorState(monitor) {
			deleteMonitorState(monitor)
			return &Report{
				Severity: severityInfo,
				Text:     fmt.Sprintf("Metrics monitor %s recovered, value %.2f < %.2f", monitor, value, monitorValue),
			}
		}
	}
	return nil
}

// tidy monitor state for unused monitors
func tidyMonitorState(metricsMonitorMap map[string]float64) {

	for monitor, value := range metricsMonitorState {
		if _, ok := metricsMonitorMap[monitor]; !ok {
			deleteMonitorState(monitor)
			continue
		}

		// Delete state if value has changed
		if value != metricsMonitorMap[monitor] {
			deleteMonitorState(monitor)
		}
	}
}

// get the monitor state
func getMonitorState(monitor string) bool {
	metricsMonitorStatelock.Lock()
	defer metricsMonitorStatelock.Unlock()
	if _, ok := metricsMonitorState[monitor]; !ok {
		return false
	}
	return true
}

// set the monitor state
func setMonitorState(monitor string, value float64) {
	metricsMonitorStatelock.Lock()
	defer metricsMonitorStatelock.Unlock()
	metricsMonitorState[monitor] = value
}

// delete an entry from the monitor state map
func deleteMonitorState(monitor string) {
	metricsMonitorStatelock.Lock()
	defer metricsMonitorStatelock.Unlock()
	delete(metricsMonitorState, monitor)
}

// convert the metric monitors to a map
func metricMonitorsToMap(metricsMonitors []MetricMonitor) map[string]float64 {
	monitorMap := make(map[string]float64)
	for _, monitor := range metricsMonitors {
		if monitor.ID != "" {
			monitorMap[monitor.Value+":"+monitor.ID] = monitor.Threshold
			continue
		}
		monitorMap[monitor.Value] = monitor.Threshold
	}
	return monitorMap
}

// convert the metrics to a map
func metricsToMap(metrics []metrics.Metric) (map[string]float64, error) {
	metricBytes, err := json.Marshal(metrics)
	if err != nil {
		return nil, err
	}

	var metricData any
	err = json.Unmarshal(metricBytes, &metricData)
	if err != nil {
		return nil, err
	}

	mapValues := make(map[string]float64)
	for _, metric := range metricData.([]any) {
		metMap := metric.(map[string]any)
		monitorLabel, ok := metMap["label"].(string)
		if !ok {
			continue
		}
		monitorid, hasId := metMap["id"].(string)

		for valueLabel, monitor := range metMap["values"].(map[string]any) {
			if hasId {
				mapValues[monitorLabel+":"+valueLabel+":"+monitorid] = monitor.(float64)
			} else {
				mapValues[monitorLabel+":"+valueLabel] = monitor.(float64)
			}
		}
	}
	return mapValues, nil
}
