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

package api

import (
	"fmt"
	"io"
)

// ConnectionError is used to explicitly indicate API connectivity issue.
// This is used to track failed API connection attempts.
type ConnectionError struct {
	err error
}

// NewConnectionError returns a new connection error with the given underlying error.
func NewConnectionError(err error) ConnectionError {
	return ConnectionError{
		err: err,
	}
}

// Error returns a string representation of the error.
func (err ConnectionError) Error() string {
	return fmt.Sprintf("failed to send API request: %s", err.err)
}

// Unwrap returns the underlying error.
func (err ConnectionError) Unwrap() error {
	return err.err
}

// Error returned when HTTP API request results in status code >= 400.
type Error struct {
	ResponseCode int
	ResponseBody []byte
}

// Error returns a string representation of the error.
func (err *Error) Error() string {
	return fmt.Sprintf("unexpected API response: %d %s", err.ResponseCode, err.ResponseBody)
}

// NewError returns a new API error with the given response status code and body.
func NewError(responseStatusCode int, responseBody io.Reader) error {
	responseBodyContents, _ := io.ReadAll(responseBody)

	return &Error{
		ResponseCode: responseStatusCode,
		ResponseBody: responseBodyContents,
	}
}
