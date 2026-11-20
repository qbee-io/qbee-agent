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

	// kill all subprocesses as well when ctx is cancelled (e.g. the execution time limit is reached)
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	// explicitly set working directory to root
	cmd.Dir = "/"

	// append tail buffer to Stdout to collect only most recent bytes
	tailBuffer := utils.NewTailBuffer(commandOutputBytesLimit)

	// The output goes through our own pipe, so cmd.Run waits only for the shell, and not for background
	// subprocesses which keep the output open. We read the output until all of them closed it.
	outputReader, outputWriter, pipeErr := os.Pipe()
	if pipeErr != nil {
		return nil, fmt.Errorf("cannot create output pipe: %w", pipeErr)
	}
	defer func() { _ = outputReader.Close() }()

	cmd.Stdout = outputWriter
	cmd.Stderr = outputWriter

	outputClosed := make(chan struct{})
	go func() {
		_, _ = io.Copy(tailBuffer, outputReader)
		close(outputClosed)
	}()

	// run the command
	err := cmd.Run()

	// from now on, only the command's subprocesses keep the output open
	_ = outputWriter.Close()

	// Background subprocesses (e.g. "sleep 2 &") may keep the output open after the shell exits.
	// The shell may have exited successfully before they were killed, so report the cancellation then.
	if waitErr := waitForSubprocesses(ctx, cmd, outputReader, outputClosed); err == nil {
		err = waitErr
	}

	// grab tail of the output
	output := tailBuffer.Close()

	return output, err
}

// waitForSubprocesses waits until all subprocesses of the command closed its output.
// If ctx is cancelled first, it kills the command's process group, stops reading the output after a second
// (for subprocesses which left the process group) and returns ctx.Err().
func waitForSubprocesses(ctx context.Context, cmd *exec.Cmd, outputReader *os.File, outputClosed <-chan struct{}) error {
	select {
	case <-outputClosed:
		return nil
	case <-ctx.Done():
	}

	// Kill the process group, including background subprocesses.
	// cmd.Process is nil if the command didn't start (e.g. ctx was cancelled before).
	// The process group ID can't be reused while any process of the group is still running.
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	// Subprocesses which left the process group (e.g. "setsid sleep 600 &") aren't killed and may keep
	// the output open, so we stop reading it after a second.
	stopReading := time.AfterFunc(time.Second, func() { _ = outputReader.Close() })
	<-outputClosed
	stopReading.Stop()

	return ctx.Err()
}
