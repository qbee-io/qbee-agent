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

package inventory

import (
	"testing"

	"go.qbee.io/agent/app/utils/assert"
)

func TestCollectProcessesInventory(t *testing.T) {
	processes, err := CollectProcessesInventory()
	if err != nil {
		t.Fatalf("error collecting processes: %v", err)
	}

	if len(processes.Processes) == 0 {
		t.Fatalf("no processes collected")
	}

	for _, process := range processes.Processes {
		assert.NotEmpty(t, process.PID)
		assert.NotEmpty(t, process.User)
		assert.NotEmpty(t, process.Command)
	}
}
