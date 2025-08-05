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
	"sync"
)

// TailBuffer keeps a size-limited tail of data in bytes.
type TailBuffer struct {
	mutex     sync.Mutex
	buffer    []byte
	maxBytes  int
}

// NewTailBuffer returns an initialized TailBuffer for maxBytes.
func NewTailBuffer(maxBytes int) *TailBuffer {
	return &TailBuffer{
		maxBytes: maxBytes,
	}
}

// Write implements io.Writer for TailBuffer.
func (tf *TailBuffer) Write(data []byte) (int, error) {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	// Append the new data to the buffer
	tf.buffer = append(tf.buffer, data...)

	// If buffer exceeds maxBytes, trim from the beginning to preserve the tail
	if len(tf.buffer) > tf.maxBytes {
		// Keep the last maxBytes, but try to preserve line boundaries
		excess := len(tf.buffer) - tf.maxBytes
		tf.buffer = tf.buffer[excess:]

		// Try to find the next newline to avoid cutting in the middle of a line
		if newlineIdx := bytes.IndexByte(tf.buffer, '\n'); newlineIdx != -1 {
			tf.buffer = tf.buffer[newlineIdx+1:]
		}
	}

	return len(data), nil
}

// Push adds a line to the end of the buffer.
// This method is kept for compatibility but now works with the byte-based buffer.
func (tf *TailBuffer) Push(line []byte) {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	// Add the line with a newline character
	if len(tf.buffer) > 0 {
		tf.buffer = append(tf.buffer, '\n')
	}
	tf.buffer = append(tf.buffer, bytes.TrimSpace(line)...)

	// If buffer exceeds maxBytes, trim from the beginning
	if len(tf.buffer) > tf.maxBytes {
		excess := len(tf.buffer) - tf.maxBytes
		tf.buffer = tf.buffer[excess:]

		// Try to find the next newline to avoid cutting in the middle of a line
		if newlineIdx := bytes.IndexByte(tf.buffer, '\n'); newlineIdx != -1 {
			tf.buffer = tf.buffer[newlineIdx+1:]
		}
	}
}

// Close the buffer and return all recorded lines.
// After calling this method, any writes will result in a panic.
func (tf *TailBuffer) Close() [][]byte {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	if len(tf.buffer) == 0 {
		return [][]byte{}
	}

	// Split the buffer into lines
	lines := bytes.Split(tf.buffer, []byte{'\n'})
	
	// Create result slice with proper capacity
	result := make([][]byte, 0, len(lines))
	
	for _, line := range lines {
		// Skip empty lines at the end (from trailing newlines)
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			result = append(result, trimmed)
		}
	}

	// Clear the buffer to prevent further use
	tf.buffer = nil

	return result
}
