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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ForLines runs fn for every line in the provided io.Reader.
func ForLines(reader io.Reader, fn func(string) error) error {
	scanner := bufio.NewScanner(reader)

	var lineNumber uint64
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading line: %w", err)
		}

		lineNumber++

		if err := fn(scanner.Text()); err != nil {
			return fmt.Errorf("error processing line %d: %w", lineNumber, err)
		}
	}

	return nil
}

// ForLinesInFile runs fn for every line in the provided filePath.
func ForLinesInFile(filePath string, fn func(string) error) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", filePath, err)
	}

	defer file.Close()

	if err = ForLines(file, fn); err != nil {
		return fmt.Errorf("error processing file %s: %w", filePath, err)
	}

	return nil
}

// ForLinesInCommandOutput executes a command and runs fn for each line of the stdout.
func ForLinesInCommandOutput(ctx context.Context, cmd []string, fn func(string) error) error {
	output, err := RunCommand(ctx, cmd)
	if err != nil {
		return err
	}

	return ForLines(bytes.NewBuffer(output), fn)
}

// ParseEnvFile parses env file into a map of strings.
func ParseEnvFile(filePath string) (map[string]string, error) {
	const expectedLineSubstrings = 2
	data := make(map[string]string)

	err := ForLinesInFile(filePath, func(line string) error {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "#") || line == "" {
			return nil
		}

		substrings := strings.SplitN(line, "=", expectedLineSubstrings)

		if len(substrings) != expectedLineSubstrings {
			return nil
		}

		var err error
		key := substrings[0]
		value := substrings[1]

		if strings.HasPrefix(value, `"`) {
			if value, err = strconv.Unquote(value); err != nil {
				return nil
			}
		}

		data[key] = value

		return nil
	})
	if err != nil {
		return nil, err
	}

	return data, nil
}
