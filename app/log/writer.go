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

package log

// Writer is a log writer.
type Writer struct {
	level  int
	prefix string
}

// Write writes the log message at the specified level and prefix.
func (w *Writer) Write(p []byte) (n int, err error) {
	if level < w.level {
		return len(p), nil
	}

	logf(w.level, "%s%s", w.prefix, p)

	return len(p), nil
}

// NewWriter returns new log writer with specified level and prefix.
func NewWriter(level int, prefix string) *Writer {
	return &Writer{
		level:  level,
		prefix: prefix,
	}
}
