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
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

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

// outputCloseTimeout is how long RunCommand waits for the output to be closed after killing the command.
const outputCloseTimeout = time.Second

// RunCommand runs a command and returns its output.
func RunCommand(ctx context.Context, command string) ([]byte, error) {
	command = resolveParameters(ctx, command)

	shell := getShell()
	if shell == "" {
		return nil, fmt.Errorf("cannot execute command '%s', no supported shell found", command)
	}

	cmd := utils.NewCommand(ctx, []string{shell, "-c", command})

	// The output goes through our own pipe, so cmd.Wait doesn't wait for it (it would also wait for
	// background subprocesses which keep the output open). Instead, we read it until all writers closed it.
	outputReader, outputWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("cannot create output pipe: %w", err)
	}
	defer func() { _ = outputReader.Close() }()

	cmd.Stdout = outputWriter
	cmd.Stderr = outputWriter

	err = cmd.Start()
	_ = outputWriter.Close()
	if err != nil {
		return nil, err
	}

	// append tail buffer to Stdout to collect only most recent bytes
	tailBuffer := utils.NewTailBuffer(commandOutputBytesLimit)

	outputClosed := make(chan struct{})
	go func() {
		_, _ = io.Copy(tailBuffer, outputReader)
		close(outputClosed)
	}()

	// When ctx is cancelled while the shell runs, cmd kills its process group (see utils.NewCommand).
	// The shell is reaped as soon as it exits, as background subprocesses may wait for it to disappear.
	err = cmd.Wait()

	killed := false

	select {
	case <-outputClosed:
	case <-ctx.Done():
		// Kill background subprocesses (e.g. "sleep 600 &") which keep the output open.
		// The process group ID can't be reused while any process of the group is still running.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		killed = true

		// subprocesses which left the process group (e.g. "setsid sleep 600 &") may still keep the output open
		select {
		case <-outputClosed:
		case <-time.After(outputCloseTimeout):
			_ = outputReader.Close()
			<-outputClosed
		}
	}

	// the shell may have exited successfully before its subprocesses were killed
	if killed && err == nil {
		err = ctx.Err()
	}

	// grab tail of the output
	output := tailBuffer.Close()

	return output, err
}
