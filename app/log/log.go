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

import "log"

// Supported logs severity levels.
const (
	ERROR = iota
	WARNING
	INFO
	DEBUG
)

var levelPrefix = map[int]string{
	ERROR:   "[ERROR] ",
	WARNING: "[WARNING] ",
	INFO:    "[INFO] ",
	DEBUG:   "[DEBUG] ",
}

var level = INFO

func logf(msgLevel int, msg string, args ...any) {
	if level < msgLevel {
		return
	}

	log.Printf(levelPrefix[msgLevel]+msg, args...)
}

// Debugf logs message with DEBUG severity.
func Debugf(msg string, args ...any) {
	logf(DEBUG, msg, args...)
}

// Infof logs message with INFO severity.
func Infof(msg string, args ...any) {
	logf(INFO, msg, args...)
}

// Warnf logs message with WARNING severity.
func Warnf(msg string, args ...any) {
	logf(WARNING, msg, args...)
}

// Errorf logs message with ERROR severity.
func Errorf(msg string, args ...any) {
	logf(ERROR, msg, args...)
}

// SetLevel sets current log level.
func SetLevel(newLevel int) {
	level = newLevel
}
