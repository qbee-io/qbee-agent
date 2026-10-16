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
	"os"
	"sync"
)

// TailBuffer keeps the last maxBytes of data written to it in a fixed-size ring buffer.
// The ring is allocated once, so writes never allocate regardless of how much data is written.
//
// Layout of the ring:
//   - Before it wraps (full == false), valid data is ring[:pos] in order.
//   - After it wraps (full == true), the whole ring is valid. pos points at the oldest byte, which
//     is also where the next write goes, so the logical order is ring[pos:] followed by ring[:pos].
type TailBuffer struct {
	mutex     sync.Mutex
	ring      []byte
	pos       int  // index of the next byte to write; once full, also the oldest byte
	full      bool // ring has been filled at least once, so every byte in it is valid
	truncated bool // some written data has been overwritten (or never fit) and is lost
	closed    bool // set by Close; further writes are rejected
}

// NewTailBuffer returns an initialized TailBuffer for maxBytes.
func NewTailBuffer(maxBytes int) *TailBuffer {
	// A zero-sized ring is valid and simply discards everything.
	if maxBytes < 0 {
		maxBytes = 0
	}

	return &TailBuffer{
		ring: make([]byte, maxBytes),
	}
}

// Write implements io.Writer for TailBuffer.
func (tf *TailBuffer) Write(data []byte) (int, error) {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	if tf.closed {
		return 0, os.ErrClosed
	}

	tf.write(data)

	// Report the full length as written even if part of it was discarded. Returning less would make
	// callers such as exec.Cmd treat it as a short write and fail the command.
	return len(data), nil
}

// Push adds a line to the end of the buffer.
func (tf *TailBuffer) Push(line []byte) {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	if tf.closed {
		return
	}

	// Separate from the previous line, but don't start an empty buffer with a newline.
	if tf.pos > 0 || tf.full {
		tf.write([]byte{'\n'})
	}
	tf.write(bytes.TrimSpace(line))
}

// write copies data into the ring, overwriting the oldest bytes. Caller must hold the mutex.
func (tf *TailBuffer) write(data []byte) {
	size := len(tf.ring)
	if size == 0 || len(data) == 0 {
		return
	}

	// The data alone fills the whole ring, so only its last size bytes survive. Store them in order
	// starting at index 0, which leaves the ring unwrapped (pos 0, oldest byte at index 0).
	if len(data) >= size {
		// Data is lost if this write is bigger than the ring, or if the ring already held anything.
		if len(data) > size || tf.pos > 0 || tf.full {
			tf.truncated = true
		}
		copy(tf.ring, data[len(data)-size:])
		tf.pos = 0
		tf.full = true
		return
	}

	// Once the ring is full, any further write overwrites the oldest bytes.
	if tf.full {
		tf.truncated = true
	}

	// Copy as much as fits between pos and the end of the ring, then wrap around and put the rest at
	// the start. Since len(data) < size, the wrapped part never reaches pos again.
	n := copy(tf.ring[tf.pos:], data)
	if n < len(data) {
		// The write ran past the end of the ring, so the rest overwrites the oldest bytes at the start.
		tf.truncated = true
		n += copy(tf.ring, data[n:])
		tf.full = true
	}

	// Advance pos, wrapping to 0 when the write ends exactly at the end of the ring.
	tf.pos = (tf.pos + n) % size
	if tf.pos == 0 {
		tf.full = true
	}
}

// Close the buffer and return the recorded tail of the data.
// If data was truncated, the leading partial line is dropped when a line boundary is available.
// After calling this method, any writes will return os.ErrClosed.
func (tf *TailBuffer) Close() []byte {
	tf.mutex.Lock()
	defer tf.mutex.Unlock()

	tf.closed = true

	// Unwrap the ring into logical order (oldest byte first).
	var output []byte
	if tf.full && tf.pos > 0 {
		// Wrapped: the oldest data starts at pos. Do the single copy to stitch the two halves together.
		output = make([]byte, len(tf.ring))
		n := copy(output, tf.ring[tf.pos:])
		copy(output[n:], tf.ring[:tf.pos])
	} else if tf.full {
		// Full but pos is 0, so the ring is already in order and can be returned as is.
		output = tf.ring
	} else {
		// Never filled: valid data is everything written so far.
		output = tf.ring[:tf.pos]
	}

	// Drop our reference so the returned slice is owned by the caller and nothing writes to it again.
	tf.ring = nil

	// If older data was lost, the first line is most likely cut in the middle. Skip to the start of
	// the next full line. Without any newline there is nothing better to keep, so return the raw tail.
	if tf.truncated {
		if newlineIdx := bytes.IndexByte(output, '\n'); newlineIdx != -1 {
			output = output[newlineIdx+1:]
		}
	}

	return output
}
