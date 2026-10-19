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

package configuration

import (
	"context"
	"io"
	"strings"

	"go.qbee.io/agent/app/utils"
)

// CheckCondition checks if the provided condition is met (eg. 0 exit code)
func CheckCondition(ctx context.Context, condition string) bool {
	condition = resolveParameters(ctx, condition)

	condition = strings.TrimSpace(condition)

	if condition == "" {
		return true
	}

	// return false if shell is not resolved
	shell := getShell()
	if shell == "" {
		return false
	}

	checkCmd := utils.NewCommand(ctx, []string{shell, "-c", condition})

	// discard any output, only care about the exit code
	checkCmd.Stderr = io.Discard
	checkCmd.Stdout = io.Discard

	return checkCmd.Run() == nil
}
