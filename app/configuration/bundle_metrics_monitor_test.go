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

package configuration_test

import (
	"context"
	"testing"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/metrics"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_Metrics_Monitor(t *testing.T) {

	r := runner.New(t)

	metricsMonitorBundle := configuration.MetricsMonitorBundle{
		Metrics: []configuration.MetricMonitor{
			{
				Value:     "memory:memutil",
				Threshold: 0.0,
			},
		},
	}

	reports := executeMetricMonitorBundle(r, metricsMonitorBundle)

	expectedReports := []string{
		"[WARN] Metrics monitor memory:memutil triggered",
	}

	assert.Equal(t, len(reports), len(expectedReports))
	assert.HasPrefix(t, reports[0], expectedReports[0])
}

func Test_Monitor_State_Set(t *testing.T) {

	metricsMonitorBundle := configuration.MetricsMonitorBundle{
		Metrics: []configuration.MetricMonitor{
			{
				Value:     "memory:memutil",
				Threshold: 0.0,
			},
		},
	}

	ctx := context.Background()

	metricsService := metrics.New(nil)
	collectedMetrics := metricsService.Collect()
	reports, err := metricsMonitorBundle.EvaluateMonitors(ctx, collectedMetrics)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.Equal(t, len(reports), 1)

	collectedMetrics = metricsService.Collect()
	reports, err = metricsMonitorBundle.EvaluateMonitors(ctx, collectedMetrics)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Monitor has not changed value, we should not get a new report
	assert.Equal(t, len(reports), 0)

}

func Test_Monitor_State_Reset(t *testing.T) {

	metricsMonitorBundle := configuration.MetricsMonitorBundle{
		Metrics: []configuration.MetricMonitor{
			{
				Value:     "memory:memutil",
				Threshold: 0.0,
			},
		},
	}

	ctx := context.Background()

	metricsService := metrics.New(nil)
	collectedMetrics := metricsService.Collect()
	reports, err := metricsMonitorBundle.EvaluateMonitors(ctx, collectedMetrics)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.Equal(t, len(reports), 1)

	metricsMonitorBundle = configuration.MetricsMonitorBundle{
		Metrics: []configuration.MetricMonitor{
			{
				Value:     "memory:memutil",
				Threshold: 1.0,
			},
		},
	}

	collectedMetrics = metricsService.Collect()
	reports, err = metricsMonitorBundle.EvaluateMonitors(ctx, collectedMetrics)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Monitor has changed value, we should get a new report
	assert.Equal(t, len(reports), 1)

}

func executeMetricMonitorBundle(r *runner.Runner, bundle configuration.MetricsMonitorBundle) []string {
	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleMetricsMonitor},
		BundleData: configuration.BundleData{
			MetricsMonitor: &bundle,
		},
	}

	config.BundleData.MetricsMonitor.Enabled = true

	r.CreateJSON("/app/config.json", config)

	output := r.MustExec("qbee-agent", "config", "-r", "-f", "/app/config.json")

	reports, _ := configuration.ParseTestConfigExecuteOutput(output)

	return reports
}
