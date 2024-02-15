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

//go:build linux

package inventory_test

import (
	"testing"

	"go.qbee.io/agent/app/inventory"
	"go.qbee.io/agent/app/utils/assert"
)

func TestCollectSystemInventory(t *testing.T) {

	systemInfo, err := inventory.CollectSystemInventory()
	if err != nil {
		t.Fatalf("error collecting system info: %v", err)
	}

	assert.NotEmpty(t, systemInfo.System.OSType)
	assert.NotEmpty(t, systemInfo.System.Flavor)
	assert.Equal(t, systemInfo.System.OS, "linux")
	assert.Equal(t, systemInfo.System.Class, "linux")
	assert.NotEmpty(t, systemInfo.System.Host)
	assert.NotEmpty(t, systemInfo.System.Architecture)
}
