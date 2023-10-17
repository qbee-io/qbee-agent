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

package utils

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// RunCommand runs a command and returns its output.
func RunCommand(ctx context.Context, cmd []string) ([]byte, error) {
	command := exec.CommandContext(ctx, cmd[0], cmd[1:]...)

	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGINT,
	}

	command.Dir = "/"

	output, err := command.Output()
	if err != nil {
		exitError := new(exec.ExitError)
		if errors.As(err, &exitError) {
			return nil, fmt.Errorf("error running command %v: %w\n%s", cmd, err, exitError.Stderr)
		}

		return nil, fmt.Errorf("error running command %v: %w", cmd, err)
	}
	return output, nil
}
