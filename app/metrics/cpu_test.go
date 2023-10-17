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
)

func TestCollectCPU(t *testing.T) {
	v, err := CollectCPU()
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}

	time.Sleep(1 * time.Second)

	v2, err := CollectCPU()
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}

	v3, err := v2.Delta(v)
	if err != nil {
		t.Fatalf("unexpected error = %v", err)
	}
	// verify that sum of values is about 100% (accounted for rounding errors)

	total := v3.User + v3.Nice + v3.System + v3.Idle + v3.IOWait + v3.IRQ
	d := 100 - total
	if d > 1 || d < -1 {
		t.Fatalf("expected total of 100%%, got %f%%", total)
	}
}
