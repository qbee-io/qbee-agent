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

//go:build windows

package remoteaccess

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"syscall"

	"github.com/UserExistsError/conpty"
	"github.com/xtaci/smux"
	"go.qbee.io/transport"
)

// Console contains resources involved in a remote console session.
type Console struct {
	id  string
	pty *conpty.ConPty
}

// Close the console and release all resources.
func (c *Console) Close() {

	if c.pty != nil {
		_ = c.pty.Close()
	}

}

// Resize the console.
func (c *Console) Resize(rows, cols uint16) error {
	return c.pty.Resize(int(cols), int(rows))
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

// NewConsole creates a new console.
// It returns a Console object and an error if any.
// If err == nil, the caller is responsible for closing the Console.
func NewConsole(ctx context.Context, rows, cols uint16) (*Console, error) {
	consoleID, err := newConsoleID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate console ID: %w", err)
	}

	console := &Console{
		id: consoleID,
	}

	sizeOption := conpty.ConPtyDimensions(int(cols), int(rows))

	if console.pty, err = conpty.Start("powershell.exe", sizeOption); err != nil {
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

	s.consoleMapMutex.Lock()
	s.consoleMap[console.id] = console
	s.consoleMapMutex.Unlock()

	defer func() {
		s.consoleMapMutex.Lock()
		delete(s.consoleMap, console.id)
		s.consoleMapMutex.Unlock()
		//console.Close()
	}()

	if err = transport.WriteOK(stream, []byte(console.id)); err != nil {
		return err
	}

	if _, _, err = console.Pipe(stream, console.pty); err != nil {
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

// Windows pty https://github.com/ActiveState/termtest/blob/master/conpty/conpty_windows.go

// Pipe copies data from src to dst and vice versa and returns first non-nil error.
func (c *Console) Pipe(src io.ReadWriteCloser, dst io.ReadWriteCloser) (sent int64, received int64, err error) {

	defer func() {
		_ = src.Close()
		_ = dst.Close()
	}()

	go func() {
		received, _ = io.Copy(src, dst)
	}()

	go func() {
		sent, _ = io.Copy(dst, src)
	}()

	ctx := context.Background()
	_, err = c.pty.Wait(ctx)

	// Ignore closed pipe and EOF errors.
	if errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.EOF) {
		err = nil
	}

	return
}
