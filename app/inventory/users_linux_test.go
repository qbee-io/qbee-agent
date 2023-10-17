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
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/qbee-io/qbee-agent/app/inventory"
)

func TestCollectUsersInventory(t *testing.T) {
	if _, err := os.ReadFile(inventory.ShadowFilePath); err != nil {
		t.Skipf("can't open /etc/shadow - skipping test: %v", err)
	}

	users, err := inventory.CollectUsersInventory()
	if err != nil {
		t.Fatalf("error collecting users inventory: %v", err)
	}

	data, _ := json.MarshalIndent(users, " ", " ")

	fmt.Println(string(data))
}
