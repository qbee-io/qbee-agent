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
