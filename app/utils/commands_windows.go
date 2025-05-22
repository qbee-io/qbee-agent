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

//go:build windows

package utils

import (
	"context"
	"fmt"
)

// RunCommand runs a command and returns its output.
func RunCommand(ctx context.Context, cmd []string) ([]byte, error) {
	return nil, nil
}

// GenerateServiceCommand generates a command to start or stop a service.
func GenerateServiceCommand(ctx context.Context, serviceName, action string) ([]string, error) {
	if action != "start" && action != "stop" {
		return nil, fmt.Errorf("invalid action: %s", action)
	}
	if action == "start" {
		return []string{"sc", "start", serviceName}, nil
	}
	if action == "stop" {
		return []string{"sc", "stop", serviceName}, nil
	}
	return nil, fmt.Errorf("unsupported action: %s", action)
}

func RebootCommand() ([]string, error) {
	return nil, fmt.Errorf("reboot command is not supported on Windows")
}
