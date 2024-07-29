package remoteaccess

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/xtaci/smux"
	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/utils"
	"go.qbee.io/transport"
)

const streamCommandTimeout = time.Minute * 5

// Command contains resources involved in a remote command session.
type Command struct {
	id  string
	cmd *exec.Cmd
}

func NewCommand(ctx context.Context, command string, cmdArgs []string) (*Command, error) {
	sessionID, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate console ID: %w", err)
	}

	cmd := utils.NewCommand(ctx, append([]string{command}, cmdArgs...))
	cmd.Env = os.Environ()

	return &Command{
		id:  sessionID,
		cmd: cmd,
	}, nil
}

func (c *Command) Close() {
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}

	if c.cmd.Process != nil {
		_, _ = c.cmd.Process.Wait()
	}
}

// HandleCommand handles a command and streams output back to the client.
func (s *Service) HandleCommand(_ context.Context, stream *smux.Stream, payload []byte) error {
	cmdPayload := new(transport.Command)
	if err := json.Unmarshal(payload, cmdPayload); err != nil {
		return transport.WriteError(stream, fmt.Errorf("failed to unmarshal command: %w", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), streamCommandTimeout)
	defer cancel()

	cmdSession, err := NewCommand(ctx, cmdPayload.Command, cmdPayload.CommandArgs)
	if err != nil {
		return transport.WriteError(stream, fmt.Errorf("failed to start command: %w", err))
	}

	s.commandMapMutex.Lock()
	s.commandMap[cmdSession.id] = cmdSession
	s.commandMapMutex.Unlock()

	defer func() {
		s.commandMapMutex.Lock()
		delete(s.commandMap, cmdSession.id)
		s.commandMapMutex.Unlock()
		cmdSession.Close()
	}()

	outputPipe, err := cmdSession.cmd.StdoutPipe()
	if err != nil {
		log.Errorf("failed to get stdout pipe: %v", err)
		return transport.WriteError(stream, fmt.Errorf("failed to get stdout pipe: %w", err))
	}
	defer outputPipe.Close()

	errPipe, err := cmdSession.cmd.StderrPipe()
	if err != nil {
		log.Errorf("failed to get stderr pipe: %v", err)
		return transport.WriteError(stream, fmt.Errorf("failed to get stderr pipe: %w", err))
	}
	defer errPipe.Close()

	cmdReader := io.MultiReader(outputPipe, errPipe)

	if err := cmdSession.cmd.Start(); err != nil {
		log.Errorf("failed to start command: %v", err)
		return transport.WriteError(stream, fmt.Errorf("failed to start command: %w", err))
	}

	if err := transport.WriteOK(stream, []byte(cmdSession.id)); err != nil {
		return err
	}

	errorChan := make(chan error)
	go func() {
		if _, err := io.Copy(stream, cmdReader); err != nil {
			errorChan <- err
		}
		errorChan <- nil
	}()

	select {
	case <-ctx.Done():
		if ctx.Err() != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return transport.WriteError(stream, fmt.Errorf("command timed out"))
			}
			return transport.WriteError(stream, ctx.Err())
		}
	case err := <-errorChan:
		if err != nil {
			return transport.WriteError(stream, err)
		}

		// Get the exit code of the command if non-zero.
		err = cmdSession.cmd.Wait()
		if err != nil {
			return transport.WriteError(stream, err)
		}

		return nil
	}
	return nil
}

// HandleReloadCommand handles the reload command.
func (s *Service) HandleCommandInterrupt(_ context.Context, stream *smux.Stream, _ []byte) error {

	return transport.WriteError(stream, fmt.Errorf("unsupported command"))
}
