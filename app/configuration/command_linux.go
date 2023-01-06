//go:build linux

package configuration

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"

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
func RunCommand(ctx context.Context, command string, deadline time.Duration) ([]byte, error) {
	shell := getShell()
	if shell == "" {
		return nil, fmt.Errorf("cannot execute command '%s', no supported shell found", command)
	}

	cmd := exec.CommandContext(ctx, shell, "-c", command)

	// set pgid, so we can terminate all subprocesses as well
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// append tail buffer to Stdout to collect only most recent lines
	tailBuffer := utils.NewTailBuffer(commandOutputLinesLimit)

	cmd.Stdout = tailBuffer
	cmd.Stderr = tailBuffer

	// send SIGKILL to the command when deadline is exceeded
	killTimer := time.AfterFunc(deadline, func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	})
	defer killTimer.Stop()

	// run the command
	err := cmd.Run()

	// grab tail of the output
	outputLines := tailBuffer.Close()
	output := bytes.Join(outputLines, []byte("\n"))

	return output, err
}
