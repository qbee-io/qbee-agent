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
	"path/filepath"
	"testing"

	"go.qbee.io/agent/app/inventory"
	"go.qbee.io/agent/app/utils/assert"
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

func TestPasswdShadowParsing(t *testing.T) {
	passwdContent := `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
garbled:line

qbee:x:1001:1001::/home/qbee:/bin/bash`

	shadowContent := `root:$6$asdlksadlkj:18698:0:99999:7:::
daemon:*:18698:0:99999:7:::

qbee:$6$asdlksadlkj:18698:0:99999:7:::`

	testDir := t.TempDir()

	passwdPath := filepath.Join(testDir, "passwd")
	shadowPath := filepath.Join(testDir, "shadow")

	expected := []inventory.User{
		{
			Name:              "root",
			UID:               0,
			GID:               0,
			GECOS:             "root",
			HomeDirectory:     "/root",
			Shell:             "/bin/bash",
			HasPassword:       "yes",
			PasswordAlgorithm: inventory.PasswordAlgorithmSHA512,
		},
		{
			Name:              "daemon",
			UID:               1,
			GID:               1,
			GECOS:             "daemon",
			HomeDirectory:     "/usr/sbin",
			Shell:             "/usr/sbin/nologin",
			HasPassword:       "no",
			PasswordAlgorithm: 0,
		},
		{
			Name:              "qbee",
			UID:               1001,
			GID:               1001,
			GECOS:             "",
			HomeDirectory:     "/home/qbee",
			Shell:             "/bin/bash",
			HasPassword:       "yes",
			PasswordAlgorithm: inventory.PasswordAlgorithmSHA512,
		},
	}

	if err := os.WriteFile(passwdPath, []byte(passwdContent), 0644); err != nil {
		t.Fatalf("error writing passwd file: %v", err)
	}

	if err := os.WriteFile(shadowPath, []byte(shadowContent), 0644); err != nil {
		t.Fatalf("error writing shadow file: %v", err)
	}

	users, err := inventory.GetUsersFromPasswd(passwdPath, shadowPath)
	if err != nil {
		t.Fatalf("error getting users from passwd: %v", err)
	}

	if len(users) != len(expected) {
		t.Fatalf("expected %d users, got %d", len(expected), len(users))
	}

	assert.Equal(t, len(users), len(expected))
}
