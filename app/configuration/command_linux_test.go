// Copyright 2026 qbee.io
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

package configuration

import (
	"context"
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

func TestRunCommand_CancelKillsSubprocesses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// the background process keeps the command's output open after the shell is killed
	start := time.Now()
	_, err := RunCommand(ctx, "sleep 30 & wait")

	assert.True(t, err != nil)
	assert.True(t, time.Since(start) < 5*time.Second)
}
