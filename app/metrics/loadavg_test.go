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
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

func TestCollectLoadAverage(t *testing.T) {
	gotMetrics, err := CollectLoadAverage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assert.Length(t, gotMetrics, 1)

	metric := gotMetrics[0]

	assert.Equal(t, metric.Label, LoadAverage)
	assert.Empty(t, metric.ID)

	if time.Since(time.Unix(metric.Timestamp, 0)) > time.Second {
		t.Fatalf("invalid timestamp, got: %v", metric.Timestamp)
	}
}
