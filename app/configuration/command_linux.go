//go:build linux

package configuration

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"syscall"

	"github.com/qbee-io/qbee-agent/app/utils"
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

const commandOutputLinesLimit = 100

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
