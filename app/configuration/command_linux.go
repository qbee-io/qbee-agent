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

	// when ctx is cancelled (e.g. the execution time limit is reached), the command is killed with all its
	// subprocesses, which also closes the output we wait for
	cmd := utils.NewCommand(ctx, []string{shell, "-c", command})

	// keep waiting for the output of background subprocesses (e.g. "sleep 2 &") after the shell exits,
	// otherwise valid commands fail when their subprocesses outlive the shell by more than WaitDelay
	cmd.WaitDelay = 0

	// append tail buffer to Stdout to collect only most recent bytes
	tailBuffer := utils.NewTailBuffer(commandOutputBytesLimit)

	cmd.Stdout = tailBuffer
	cmd.Stderr = tailBuffer

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// exec.Cmd stops watching ctx when the shell exits, but its background subprocesses may keep running
	// and keep the output open, so we keep watching ctx until the output is closed
	outputClosed := make(chan struct{})
	killed := make(chan bool, 1)

	go func() {
		select {
		case <-ctx.Done():
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			killed <- true
		case <-outputClosed:
			killed <- false
		}
	}()

	err := cmd.Wait()
	close(outputClosed)

	// the shell may have exited successfully before its subprocesses were killed
	if <-killed && err == nil {
		err = ctx.Err()
	}

	// grab tail of the output
	output := tailBuffer.Close()

	return output, err
}
