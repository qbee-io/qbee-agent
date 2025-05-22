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

package configuration

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"go.qbee.io/agent/app/utils"
)

const commandOutputLinesLimit = 100

func RunCommand(ctx context.Context, command string) ([]byte, error) {
	command = resolveParameters(ctx, command)

	shell := getShell()
	if shell == "" {
		return nil, fmt.Errorf("cannot execute command '%s', no supported shell found", command)
	}

	cmd := exec.Command(shell, "-Command ", command)

	// set pgid, so we can terminate all subprocesses as well

	// explicitly set working directory to root
	cmd.Dir = `C:\`

	// append tail buffer to Stdout to collect only most recent lines
	tailBuffer := utils.NewTailBuffer(commandOutputLinesLimit)

	cmd.Stdout = tailBuffer
	cmd.Stderr = tailBuffer

	// run the command
	err := cmd.Run()
	// grab tail of the output
	outputLines := tailBuffer.Close()
	output := bytes.Join(outputLines, []byte("\n"))

	return output, err
}

func getShell() string {
	return "powershell.exe"
}
