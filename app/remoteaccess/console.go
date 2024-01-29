package remoteaccess

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"github.com/xtaci/smux"
	"go.qbee.io/transport"

	"go.qbee.io/agent/app/inventory"
	"go.qbee.io/agent/app/utils"
)

// Console contains resources involved in a remote console session.
type Console struct {
	id  string
	cmd *exec.Cmd
	pty *os.File
}

// Close the console and release all resources.
func (c *Console) Close() {
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}

	if c.pty != nil {
		_ = c.pty.Close()
	}
}

// Resize the console.
func (c *Console) Resize(rows, cols uint16) error {
	return pty.Setsize(c.pty, &pty.Winsize{Rows: rows, Cols: cols})
}

// newConsoleID generates a new random console ID.
func newConsoleID() (string, error) {
	buf := make([]byte, 16)

	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}

	consoleID := base64.URLEncoding.EncodeToString(buf)

	return consoleID, nil
}

// getDefaultShell returns the default shell for the system.
func getDefaultShell() string {
	var shell string

	_ = utils.ForLinesInFile("/etc/shells", func(line string) error {
		if strings.HasPrefix(line, "#") || line == "" {
			return nil
		}

		if shell != "" {
			return nil
		}

		shell = line

		return nil
	})

	if shell != "" {
		return shell
	}

	// if shell is still not determined, use /bin/sh.
	return "/bin/sh"
}

// getCurrentUserShell returns the current user's shell.
func getCurrentUserShell() string {
	// Try to get the shell from the SHELL environment variable if set.
	shell := os.Getenv("SHELL")
	if shell != "" {
		return shell
	}

	// if not set, try to get the shell from the passwd file for the current user.
	currentUID := os.Getuid()

	_ = utils.ForLinesInFile(inventory.PasswdFilePath, func(line string) error {
		fields := strings.Split(line, ":")

		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil
		}

		if uid == currentUID {
			shell = fields[6]
		}

		return nil
	})

	if shell != "" {
		return shell
	}

	return getDefaultShell()
}

// NewConsole creates a new console.
// It returns a Console object and an error if any.
// If err == nil, the caller is responsible for closing the Console.
func NewConsole(ctx context.Context, rows, cols uint16) (*Console, error) {
	consoleID, err := newConsoleID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate console ID: %w", err)
	}

	console := &Console{
		id:  consoleID,
		cmd: exec.CommandContext(ctx, getCurrentUserShell()),
	}

	console.cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	console.cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGHUP,
	}

	winSize := &pty.Winsize{Rows: rows, Cols: cols}

	if console.pty, err = pty.StartWithSize(console.cmd, winSize); err != nil {
		console.Close()
		return nil, fmt.Errorf("failed to start command: %v", err)
	}

	return console, nil
}

// HandleConsole handles a remote console stream.
func (s *Service) HandleConsole(ctx context.Context, stream *smux.Stream, payload []byte) error {
	initCmd := new(transport.PTYCommand)
	if err := json.Unmarshal(payload, initCmd); err != nil {
		return transport.WriteError(stream, fmt.Errorf("failed to unmarshal initial PTY command: %w", err))
	}

	if initCmd.Type != transport.PTYCommandTypeResize {
		return transport.WriteError(stream, fmt.Errorf("invalid initial PTY command type"))
	}

	console, err := NewConsole(ctx, initCmd.Rows, initCmd.Cols)
	if err != nil {
		return transport.WriteError(stream, fmt.Errorf("failed to start console: %w", err))
	}

	go func() {
		_ = console.cmd.Wait()
		console.Close()
	}()

	s.consoleMapMutex.Lock()
	s.consoleMap[console.id] = console
	s.consoleMapMutex.Unlock()

	defer func() {
		s.consoleMapMutex.Lock()
		delete(s.consoleMap, console.id)
		s.consoleMapMutex.Unlock()
		console.Close()
	}()

	if err = transport.WriteOK(stream, []byte(console.id)); err != nil {
		return err
	}

	if _, _, err = transport.Pipe(stream, console.pty); err != nil {
		// handle expected read /dev/ptmx: input/output error
		if errors.Is(err, syscall.EIO) {
			return nil
		}

		return fmt.Errorf("failed to pipe console stream: %w", err)
	}

	return nil
}

// HandleConsoleCommand handles a remote console command.
// This is primary used to resize an existing console.
func (s *Service) HandleConsoleCommand(_ context.Context, stream *smux.Stream, payload []byte) error {
	cmd := new(transport.PTYCommand)
	if err := json.Unmarshal(payload, cmd); err != nil {
		return transport.WriteError(stream, fmt.Errorf("failed to unmarshal PTY command: %w", err))
	}

	switch cmd.Type {
	case transport.PTYCommandTypeResize:
		s.consoleMapMutex.Lock()
		console, ok := s.consoleMap[cmd.SessionID]
		s.consoleMapMutex.Unlock()

		if !ok {
			return transport.WriteError(stream, fmt.Errorf("console session not found"))
		}

		if err := console.Resize(cmd.Rows, cmd.Cols); err != nil {
			return transport.WriteError(stream, fmt.Errorf("failed to resize console: %w", err))
		}

		return transport.WriteOK(stream, nil)
	default:
		return transport.WriteError(stream, fmt.Errorf("unsupported PTY command: %v", cmd.Type))
	}
}
