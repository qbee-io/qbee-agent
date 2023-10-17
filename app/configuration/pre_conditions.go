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
	"strings"

	"github.com/qbee-io/qbee-agent/app/utils"
)

// CheckPreCondition checks if the provided pre-condition is met.
func CheckPreCondition(ctx context.Context, preCondition string) bool {
	preCondition = resolveParameters(ctx, preCondition)

	preCondition = strings.TrimSpace(preCondition)

	if preCondition == "" {
		return true
	}

	// return with no error when pre-condition fails
	if _, err := utils.RunCommand(ctx, []string{getShell(), "-c", preCondition}); err != nil {
		return false
	}

	return true
}
