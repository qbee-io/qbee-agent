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

package configuration

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"syscall"

	"go.qbee.io/agent/app/utils"
)

var supportedShells = []string{"bash", "zsh", "sh"}

// getShell
func getShell() string {
	for _, shell := range supportedShells {
		if shellPath, err := exec.LookPath(shell); err == nil {
			return shellPath
		}
	}

	return ""
}

const commandOutputBytesLimit = 1024 * 1024 // 1MB

// RunCommand runs a command and returns its output.
func RunCommand(ctx context.Context, command string) ([]byte, error) {
	command = resolveParameters(ctx, command)

	shell := getShell()
	if shell == "" {
		return nil, fmt.Errorf("cannot execute command '%s', no supported shell found", command)
	}

	cmd := exec.CommandContext(ctx, shell, "-c", command)

	// set pgid, so we can terminate all subprocesses as well
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGINT,
	}

	// explicitly set working directory to root
	cmd.Dir = "/"

	// append tail buffer to Stdout to collect only most recent bytes
	tailBuffer := utils.NewTailBuffer(commandOutputBytesLimit)

	cmd.Stdout = tailBuffer
	cmd.Stderr = tailBuffer

	// run the command
	err := cmd.Run()

	// grab tail of the output
	outputLines := tailBuffer.Close()
	output := bytes.Join(outputLines, []byte("\n"))

	return output, err
}
