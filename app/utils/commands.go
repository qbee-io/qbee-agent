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
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

type cmdCtxKey int

const (
	ContextKeyElevationCommand cmdCtxKey = iota
)

// RunCommand runs a command and returns its output.
func RunCommand(ctx context.Context, cmd []string) ([]byte, error) {
	return RunCommandOutput(NewCommand(ctx, cmd))
}

// NewPrivilegedCommand creates a new exec.Cmd with privilege elevation if needed.
func NewPrivilegedCommand(ctx context.Context, cmd []string) (*exec.Cmd, error) {
	// if already root, run command directly without elevation
	if os.Geteuid() == 0 {
		return NewCommand(ctx, cmd), nil
	}

	// get elevation command from context if not provided
	elevationCmdFromCtx := ctx.Value(ContextKeyElevationCommand)
	if elevationCmdFromCtx == nil {
		return NewCommand(ctx, cmd), nil
	}
	// assert type, must be []string, return regular command if not
	elevationCmd, ok := elevationCmdFromCtx.([]string)
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

// RunPrivilegedCommand runs a command with the configured elevation command and returns its output.
func RunPrivilegedCommand(ctx context.Context, cmd []string) ([]byte, error) {
	command, err := NewPrivilegedCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return RunCommandOutput(command)
}

// RunCommandOutput runs a command and returns its output as a string.
func RunCommandOutput(command *exec.Cmd) ([]byte, error) {
	output, err := command.Output()

	if err != nil {
		exitError := new(exec.ExitError)
		if errors.As(err, &exitError) {
			return nil, fmt.Errorf("error running command %v: %w\n%s", command.Args, err, exitError.Stderr)
		}

		return nil, fmt.Errorf("error running command %v: %w", command.Args, err)
	}
	return output, nil
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
	if output, err = RunCommand(ctx, cmd); err != nil {
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
