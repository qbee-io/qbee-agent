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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/qbee-io/qbee-agent/app/api"
)

func TestCollectServiceCollectAll(t *testing.T) {
	apiClient, _ := api.NewMockedClient()

	srv := New(apiClient)

	gotMetrics := srv.Collect()

	if len(gotMetrics) == 0 {
		t.Fatalf("expected at least one metric, got 0")
	}

	metricBytes, err := json.MarshalIndent(gotMetrics, "", "  ")
	if err != nil {
		t.Fatalf("error marshaling metrics: %v", err)
	}

	fmt.Printf("got metrics: %s", string(metricBytes))

	// Sleep to get deltas
	time.Sleep(1 * time.Second)

	gotMetrics = srv.Collect()
	metricBytes, err = json.MarshalIndent(gotMetrics, "", "  ")

	if err != nil {
		t.Fatalf("error marshaling metrics: %v", err)
	}

	fmt.Printf("got metrics: %s", string(metricBytes))
}
