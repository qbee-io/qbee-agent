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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"time"
)

type cmdCtxKey int

const (
	contextKeyElevationCommand cmdCtxKey = iota
)

// RunCommand runs a command and returns its stdout, for parsing into structured data.
// Output is bounded to parseOutputBytesLimit and ErrOutputLimitExceeded is returned if it's exceeded.
// okExitCode lists exit codes treated as success (default: 0).
func RunCommand(ctx context.Context, cmd []string, okExitCode ...int) ([]byte, error) {
	return runCommandParsed(NewCommand(ctx, cmd), okExitCode)
}

// NewPrivilegedCommand creates a new exec.Cmd with privilege elevation if needed.
func NewPrivilegedCommand(ctx context.Context, cmd []string) (*exec.Cmd, error) {
	// if already root, run command directly without elevation
	if os.Geteuid() == 0 {
		return NewCommand(ctx, cmd), nil
	}

	elevationCmd, ok := GetElevationCommandFromContext(ctx)
	if !ok {
		return NewCommand(ctx, cmd), nil
	}

	// no elevation command provided, assume capabilities are set
	if len(elevationCmd) == 0 {
		return NewCommand(ctx, cmd), nil
	}

	// check if elevation command exists
	if _, err := exec.LookPath(elevationCmd[0]); err != nil {
		return nil, fmt.Errorf("%s not found: %w", elevationCmd[0], err)
	}

	cmd = append(elevationCmd, cmd...)
	return NewCommand(ctx, cmd), nil
}

// RunPrivilegedCommand runs a command with the configured elevation command and returns its stdout,
// for parsing into structured data. See RunCommand for output bounds and okExitCode.
func RunPrivilegedCommand(ctx context.Context, cmd []string, okExitCode ...int) ([]byte, error) {
	command, err := NewPrivilegedCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return runCommandParsed(command, okExitCode)
}

// RunCommandReport runs a command and returns the tail of its combined stdout/stderr output, for
// recording as part of a report. Output is bounded to reportOutputBytesLimit by retaining only the
// tail; this never produces an error on its own. An error is returned only when the command's exit
// code isn't one of okExitCode (default: 0).
func RunCommandReport(ctx context.Context, cmd []string, okExitCode ...int) ([]byte, error) {
	return runCommandReport(NewCommand(ctx, cmd), okExitCode)
}

// RunPrivilegedCommandReport runs a command with the configured elevation command and returns the tail
// of its combined stdout/stderr output. See RunCommandReport for output bounds and okExitCode.
func RunPrivilegedCommandReport(ctx context.Context, cmd []string, okExitCode ...int) ([]byte, error) {
	command, err := NewPrivilegedCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return runCommandReport(command, okExitCode)
}

// parseOutputBytesLimit bounds stdout returned from RunCommand/RunPrivilegedCommand.
const parseOutputBytesLimit = 1024 * 1024 // 1MB

// reportOutputBytesLimit bounds the combined stdout/stderr tail returned from RunCommandReport/
// RunPrivilegedCommandReport.
const reportOutputBytesLimit = 8 * 1024 * 1024 // 8MB

// ErrOutputLimitExceeded is returned when a command's stdout exceeds parseOutputBytesLimit.
var ErrOutputLimitExceeded = errors.New("command output exceeds limit")

// resolveOkExitCodes returns okExitCode, or []int{0} when none are provided.
func resolveOkExitCodes(okExitCode []int) []int {
	if len(okExitCode) == 0 {
		return []int{0}
	}
	return okExitCode
}

// isOkExitCode reports whether err is an *exec.ExitError with a code in okExitCodes.
func isOkExitCode(err error, okExitCodes []int) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	return slices.Contains(okExitCodes, exitErr.ExitCode())
}

// limitedBuffer keeps up to limit bytes and discards the rest, while still draining the writer.
type limitedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (lb *limitedBuffer) Write(data []byte) (int, error) {
	if remaining := lb.limit - lb.buffer.Len(); len(data) > remaining {
		lb.buffer.Write(data[:remaining])
		lb.exceeded = true
	} else {
		lb.buffer.Write(data)
	}
	return len(data), nil
}

// runCommandParsed runs command, capturing stdout (bounded to parseOutputBytesLimit, hard failure on
// overflow) and stderr (same bound, for diagnostics only) for structured-data use cases.
func runCommandParsed(command *exec.Cmd, okExitCode []int) ([]byte, error) {
	okExitCodes := resolveOkExitCodes(okExitCode)

	stdout := &limitedBuffer{limit: parseOutputBytesLimit}
	stderr := &limitedBuffer{limit: parseOutputBytesLimit}

	command.Stdout = stdout
	command.Stderr = stderr

	err := command.Run()
	output := stdout.buffer.Bytes()

	if err == nil || isOkExitCode(err, okExitCodes) {
		if stdout.exceeded {
			return output, fmt.Errorf("error running command %v: %w", command.Args, ErrOutputLimitExceeded)
		}
		return output, nil
	}

	if stdout.exceeded {
		err = errors.Join(err, ErrOutputLimitExceeded)
	}

	var parts [][]byte
	if trimmed := bytes.TrimSpace(output); len(trimmed) > 0 {
		parts = append(parts, trimmed)
	}
	if trimmed := bytes.TrimSpace(stderr.buffer.Bytes()); len(trimmed) > 0 {
		parts = append(parts, trimmed)
	}

	if len(parts) > 0 {
		return output, fmt.Errorf("error running command %v: %w\n%s", command.Args, err, bytes.Join(parts, []byte("\n")))
	}

	return output, fmt.Errorf("error running command %v: %w", command.Args, err)
}

// runCommandReport runs command, capturing a tail of the combined stdout/stderr output bounded to
// reportOutputBytesLimit, for recording as part of a report.
func runCommandReport(command *exec.Cmd, okExitCode []int) ([]byte, error) {
	okExitCodes := resolveOkExitCodes(okExitCode)

	tailBuffer := NewTailBuffer(reportOutputBytesLimit)
	command.Stdout = tailBuffer
	command.Stderr = tailBuffer

	err := command.Run()
	output := bytes.Join(tailBuffer.Close(), []byte("\n"))

	if err == nil || isOkExitCode(err, okExitCodes) {
		return output, nil
	}

	// the output is also returned separately, but callers often only forward the error (e.g. to a
	// report), so include it there too.
	if len(output) > 0 {
		return output, fmt.Errorf("error running command %v: %w\n%s", command.Args, err, output)
	}

	return output, fmt.Errorf("error running command %v: %w", command.Args, err)
}

// RunCommandOutput runs a pre-built command and returns its stdout, for parsing into structured data.
// See RunCommand for output bounds.
func RunCommandOutput(command *exec.Cmd) ([]byte, error) {
	return runCommandParsed(command, nil)
}

// RunCommandReportOutput runs a pre-built command and returns the tail of its combined stdout/stderr
// output, for recording as part of a report. See RunCommandReport for output bounds and okExitCode.
func RunCommandReportOutput(command *exec.Cmd, okExitCode ...int) ([]byte, error) {
	return runCommandReport(command, okExitCode)
}

// RunCommandCheck runs a command for its exit status only, discarding stdout and stderr without
// allocating any buffer for them. Use this when only the exit code matters, e.g. conditions.
func RunCommandCheck(ctx context.Context, cmd []string) error {
	command := NewCommand(ctx, cmd)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run()
}

// NewCommand creates a new exec.Cmd with the given context and command.
func NewCommand(ctx context.Context, cmd []string) *exec.Cmd {
	command := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	command.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGINT,
	}
	command.Dir = "/"
	command.WaitDelay = 1 * time.Second

	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		// kill the process group
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	return command
}

// GenerateServiceCommand generates a service command based on the service name and command
func GenerateServiceCommand(ctx context.Context, serviceName, command string) ([]string, error) {
	// up%s is only used on linux
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	// first check for systemd
	if _, err := exec.LookPath("systemctl"); err == nil {
		return generateSystemctlCommand(ctx, serviceName, command)
	}
	// then check for sysvinit
	if _, err := exec.LookPath("service"); err == nil {
		return []string{"service", serviceName, command}, nil
	}
	// then check for openrc
	if _, err := exec.LookPath("rc-service"); err == nil {
		return []string{"rc-service", serviceName, command}, nil
	}
	// then check for upstart
	if _, err := exec.LookPath("initctl"); err == nil {
		return []string{"initctl", command, serviceName}, nil
	}
	// then check for runit
	if _, err := exec.LookPath("sv"); err == nil {
		return []string{"sv", command, serviceName}, nil
	}
	// then check for launchctl
	if _, err := exec.LookPath("launchctl"); err == nil {
		return []string{"launchctl", command, serviceName}, nil
	}
	// then check for rcctl
	if _, err := exec.LookPath("rcctl"); err == nil {
		return []string{"rcctl", command, serviceName}, nil
	}
	// then check existence of /etc/init.d/qbee-agent
	if _, err := exec.LookPath(fmt.Sprintf("/etc/init.d/%s", serviceName)); err == nil {
		return []string{fmt.Sprintf("/etc/init.d/%s", serviceName), command}, nil
	}
	return nil, fmt.Errorf("unsupported service manager")
}

// generateSystemctlCommand generates a systemctl command based on the service name and command
func generateSystemctlCommand(ctx context.Context, serviceName, command string) ([]string, error) {
	serviceUnit := fmt.Sprintf("%s.service", serviceName)

	cmd := []string{"systemctl", "show", "--property=LoadState", serviceUnit}

	var output []byte
	var err error
	if output, err = RunPrivilegedCommand(ctx, cmd); err != nil {
		return nil, fmt.Errorf("error checking service status: %w", err)
	}

	// if service is not loaded, there isn't anything to restart
	if !bytes.Equal(bytes.TrimSpace(output), []byte("LoadState=loaded")) {
		return nil, nil
	}

	if command == "start" {
		return []string{"systemctl", command, serviceUnit}, nil
	}

	if serviceName == "qbee-agent" {
		return []string{"systemctl", "--no-block", command, serviceUnit}, nil
	}

	return []string{"systemctl", command, serviceUnit}, nil
}

const shutdownBinPath = "/sbin/shutdown"
const rebootBinPath = "/sbin/reboot"

// RebootCommand returns the command to reboot the system
func RebootCommand() ([]string, error) {
	if _, err := exec.LookPath(shutdownBinPath); err == nil {
		return []string{shutdownBinPath, "-r", "+1"}, nil
	}

	if _, err := exec.LookPath(rebootBinPath); err == nil {
		return []string{rebootBinPath}, nil
	}

	return nil, fmt.Errorf("cannot reboot: %s or %s not found", shutdownBinPath, rebootBinPath)
}

// ContextWithElevationCommand returns a new context with the given elevation command
func ContextWithElevationCommand(ctx context.Context, elevationCmd []string) (context.Context, error) {
	// check if elevation command exists
	if len(elevationCmd) == 0 {
		return ctx, nil
	}

	if err := ValidateElevationCommand(elevationCmd); err != nil {
		return nil, err
	}

	return context.WithValue(ctx, contextKeyElevationCommand, elevationCmd), nil
}

// GetElevationCommandFromContext retrieves the elevation command from the context
func GetElevationCommandFromContext(ctx context.Context) ([]string, bool) {
	elevationCmdFromCtx := ctx.Value(contextKeyElevationCommand)
	if elevationCmdFromCtx == nil {
		return nil, false
	}
	elevationCmd, ok := elevationCmdFromCtx.([]string)
	return elevationCmd, ok
}

// ValidateElevationCommand ensures the elevation command is safe to use.
func ValidateElevationCommand(cmd []string) error {
	// Otherwise require an absolute path to avoid PATH-based attacks.
	if !filepath.IsAbs(cmd[0]) {
		return fmt.Errorf("elevation command %q must be an absolute path", cmd[0])
	}

	// check that the command is executable
	if fileInfo, err := os.Stat(cmd[0]); err != nil {
		return fmt.Errorf("cannot stat elevation command %q: %w", cmd[0], err)
	} else if fileInfo.Mode()&0111 == 0 {
		return fmt.Errorf("elevation command %q is not executable", cmd[0])
	}

	return nil
}
