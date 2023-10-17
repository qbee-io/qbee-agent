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

// TailBuffer keeps a size-limited tail of lines.
type TailBuffer struct {
	mutex       sync.Mutex
	writeBuffer []byte
	tail        chan []byte
}

// NewTailBuffer returns an initialized TailBuffer for maxLines.
func NewTailBuffer(maxLines int) *TailBuffer {
	return &TailBuffer{
		tail: make(chan []byte, maxLines),
	}
}

// Write implements io.Writer for TailBuffer.
func (tf *TailBuffer) Write(data []byte) (int, error) {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	tf.writeBuffer = append(tf.writeBuffer, data...)

	// process all new lines
	for {
		i := bytes.IndexByte(tf.writeBuffer, '\n')
		if i < 0 {
			break
		}

		// push the line to the buffer
		tf.Push(tf.writeBuffer[0:i])
		tf.writeBuffer = tf.writeBuffer[i+1:]
	}

	return len(data), nil
}

// Push adds a line to the end of the buffer.
// If buffer is full, the first line of the buffer gets dropped.
func (tf *TailBuffer) Push(line []byte) {
	// drop the oldest line from the buffer if full
	if len(tf.tail) == cap(tf.tail) {
		<-tf.tail
	}

	tf.tail <- bytes.TrimSpace(line)
}

// Close the buffer and return all recorded lines.
// After calling this method, any writes will result in a panic.
func (tf *TailBuffer) Close() [][]byte {
	// push whatever is in the write buffer to the tail
	if len(tf.writeBuffer) > 0 {
		tf.Push(tf.writeBuffer)
		tf.writeBuffer = nil
	}

	// close the channel
	close(tf.tail)

	// collect the lines
	lines := make([][]byte, 0, len(tf.tail))

	for line := range tf.tail {
		lines = append(lines, line)
	}

	return lines
}
