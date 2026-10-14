// Copyright 2024 qbee.io
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
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCommandTimeout(t *testing.T) {

	tests := []struct {
		name          string
		command       []string
		expectedError string
	}{
		{
			name:          "command runs successfully",
			command:       []string{"echo", "hello"},
			expectedError: "",
		},
		{
			name:          "simple command killed by timeout",
			command:       []string{"sleep", "1"},
			expectedError: "signal: killed",
		},
		{
			name:          "child process killed by timeout",
			command:       []string{"sh", "-c", `(trap 'echo "cleanup completed"; exit' TERM; sleep 1)`},
			expectedError: "signal: killed",
		},
		{
			name:          "endless loop killed by timeout",
			command:       []string{"sh", "-c", "while true; do echo 'hello'; sleep 1; done"},
			expectedError: "signal: killed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Millisecond*100))
			defer cancel()

			start := time.Now()
			_, err := RunCommand(ctx, tt.command)
			if err == nil && tt.expectedError == "" {
				return
			}

			elapsed := time.Since(start)
			if elapsed.Seconds() > 1 {
				t.Fatalf("%s: unexpected elapsed time: %v", tt.name, elapsed)
			}

			if err == nil {
				t.Fatalf("%s: expected error", tt.name)
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Fatalf("%s: unexpected error: %v", tt.name, err)
			}
		})
	}
}

func TestParseCommandLine(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected []string
	}{
		{
			name:     "simple command",
			command:  "echo hello",
			expected: []string{"echo", "hello"},
		},
		{
			name:     "command with quotes",
			command:  "echo 'hello world'",
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "command with escaped quotes",
			command:  "echo \"hello world\"",
			expected: []string{"echo", "hello world"},
		},
		{
			name:     "command with escaped quotes and spaces",
			command:  "echo \"hello world\" 'and more'",
			expected: []string{"echo", "hello world", "and more"},
		},
		{
			name:     "command with flags",
			command:  "echo -n 'hello world'",
			expected: []string{"echo", "-n", "hello world"},
		},
		{
			name:     "parse empty command",
			command:  "",
			expected: []string{},
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			result, err := ParseCommandLine(tt.command)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", tt.name, err)
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("%s: unexpected result length: %v", tt.name, result)
			}

			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Fatalf("%s: unexpected result: %v", tt.name, result)
				}
			}

		})
	}
}

func TestRunCommand(t *testing.T) {
	ctx := context.Background()

	t.Run("success returns stdout and no error", func(t *testing.T) {
		out, err := RunCommand(ctx, []string{"sh", "-c", "echo 'success stdout'"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.TrimSpace(string(out)) != "success stdout" {
			t.Fatalf("unexpected output: %s", string(out))
		}
	})

	t.Run("failure with stdout and stderr includes both in error and returns stdout", func(t *testing.T) {
		out, err := RunCommand(ctx, []string{"sh", "-c", "echo 'failed stdout'; echo 'failed stderr' >&2; exit 1"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if strings.TrimSpace(string(out)) != "failed stdout" {
			t.Fatalf("unexpected output: %s", string(out))
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "failed stdout") {
			t.Fatalf("expected error to contain stdout, got: %s", errStr)
		}
		if !strings.Contains(errStr, "failed stderr") {
			t.Fatalf("expected error to contain stderr, got: %s", errStr)
		}
		// verify stdout precedes stderr in error string
		stdoutIdx := strings.Index(errStr, "failed stdout")
		stderrIdx := strings.Index(errStr, "failed stderr")
		if stdoutIdx >= stderrIdx {
			t.Fatalf("expected stdout before stderr in error, got: %s", errStr)
		}
	})

	t.Run("failure with only stdout includes stdout in error and returns stdout", func(t *testing.T) {
		out, err := RunCommand(ctx, []string{"sh", "-c", "echo 'only stdout'; exit 1"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if strings.TrimSpace(string(out)) != "only stdout" {
			t.Fatalf("unexpected output: %s", string(out))
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "only stdout") {
			t.Fatalf("expected error to contain stdout, got: %s", errStr)
		}
	})

	t.Run("failure with only stderr includes stderr in error", func(t *testing.T) {
		out, err := RunCommand(ctx, []string{"sh", "-c", "echo 'only stderr' >&2; exit 1"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if len(out) != 0 {
			t.Fatalf("expected empty output, got: %s", string(out))
		}
		errStr := err.Error()
		if !strings.Contains(errStr, "only stderr") {
			t.Fatalf("expected error to contain stderr, got: %s", errStr)
		}
	})

	t.Run("failure with verbose stdout bounds error and stdout, returns ErrOutputLimitExceeded", func(t *testing.T) {
		out, err := RunCommand(ctx, []string{"sh", "-c", "seq 1 300000; exit 1"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !errors.Is(err, ErrOutputLimitExceeded) {
			t.Fatalf("expected ErrOutputLimitExceeded, got: %v", err)
		}
		if len(out) != parseOutputBytesLimit {
			t.Fatalf("expected stdout bounded to %d bytes, got %d bytes", parseOutputBytesLimit, len(out))
		}
	})

	t.Run("success with stdout exceeding the limit returns ErrOutputLimitExceeded", func(t *testing.T) {
		out, err := RunCommand(ctx, []string{"sh", "-c", "yes | head -c $((1024*1024+1))"})
		if !errors.Is(err, ErrOutputLimitExceeded) {
			t.Fatalf("expected ErrOutputLimitExceeded, got: %v", err)
		}
		if len(out) != parseOutputBytesLimit {
			t.Fatalf("expected stdout bounded to %d bytes, got %d bytes", parseOutputBytesLimit, len(out))
		}
	})

	t.Run("okExitCode treats the listed exit code as success", func(t *testing.T) {
		out, err := RunCommand(ctx, []string{"sh", "-c", "echo 'accepted'; exit 42"}, 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.TrimSpace(string(out)) != "accepted" {
			t.Fatalf("unexpected output: %s", string(out))
		}
	})

	t.Run("failure with no output has no trailing newline in error", func(t *testing.T) {
		out, err := RunCommand(ctx, []string{"sh", "-c", "exit 1"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if len(out) != 0 {
			t.Fatalf("expected empty output, got: %s", string(out))
		}
		errStr := err.Error()
		if strings.HasSuffix(errStr, "\n") {
			t.Fatalf("expected no trailing newline in error, got: %q", errStr)
		}
	})
}

func TestRunCommandReport(t *testing.T) {
	ctx := context.Background()

	t.Run("success returns combined output and no error", func(t *testing.T) {
		out, err := RunCommandReport(ctx, []string{"sh", "-c", "echo 'report stdout'"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.TrimSpace(string(out)) != "report stdout" {
			t.Fatalf("unexpected output: %s", string(out))
		}
	})

	t.Run("failure returns combined stdout/stderr and an error", func(t *testing.T) {
		out, err := RunCommandReport(ctx, []string{"sh", "-c", "echo 'report stdout'; echo 'report stderr' >&2; exit 1"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(string(out), "report stdout") || !strings.Contains(string(out), "report stderr") {
			t.Fatalf("expected combined output, got: %s", string(out))
		}
		// the error itself must also carry the output, since callers often only forward the error.
		errStr := err.Error()
		if !strings.Contains(errStr, "report stdout") || !strings.Contains(errStr, "report stderr") {
			t.Fatalf("expected error to contain combined output, got: %s", errStr)
		}
	})

	t.Run("verbose output is bounded to the tail without an error", func(t *testing.T) {
		out, err := RunCommandReport(ctx, []string{"sh", "-c", "seq 1 3000000"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) > reportOutputBytesLimit {
			t.Fatalf("expected output bounded to %d bytes, got %d bytes", reportOutputBytesLimit, len(out))
		}
		if !strings.HasSuffix(string(out), "3000000") {
			t.Fatalf("expected tail of output to be retained, got: %s", string(out)[len(out)-100:])
		}
	})

	t.Run("okExitCode treats the listed exit code as success", func(t *testing.T) {
		out, err := RunCommandReport(ctx, []string{"sh", "-c", "echo 'accepted'; exit 42"}, 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.TrimSpace(string(out)) != "accepted" {
			t.Fatalf("unexpected output: %s", string(out))
		}
	})
}

func TestRunCommandCheck(t *testing.T) {
	ctx := context.Background()

	t.Run("success returns no error", func(t *testing.T) {
		if err := RunCommandForCheck(ctx, []string{"sh", "-c", "echo 'ignored'; exit 0"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("failure returns an error with no output captured", func(t *testing.T) {
		err := RunCommandForCheck(ctx, []string{"sh", "-c", "echo 'ignored stdout'; echo 'ignored stderr' >&2; exit 1"})
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if strings.Contains(err.Error(), "ignored") {
			t.Fatalf("expected no output in error, got: %v", err)
		}
	})
}
