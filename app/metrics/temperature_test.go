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

package metrics

import (
	"encoding/json"
	"testing"
)

func TestCollectTemperature(t *testing.T) {

	metrics, err := CollectTemperature()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, metric := range metrics {
		if metric.Label != Temperature {
			t.Errorf("unexpected label: %v", metric.Label)
		}
		if metric.Timestamp < 0 {
			t.Errorf("unexpected timestamp: %v", metric.Timestamp)
		}
		if metric.Values.Temperature == 0 {
			t.Errorf("unexpected temperature: %v", metric.Values.Temperature)
		}
	}

	_, err = hwMonTemperatureMetrics()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m3, err := thermalZoneTemperatureMetrics()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = json.MarshalIndent(m3, "", "  ")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}