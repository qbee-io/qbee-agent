// Copyright 2026 qbee.io
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
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

// roundTripFunc lets a function act as the HTTP transport of the client.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

// newRecordingClient returns a client whose requests are answered with an empty JSON object.
// The context of each request is sent to the returned channel.
func newRecordingClient() (*Client, <-chan context.Context) {
	contexts := make(chan context.Context, 1)

	cli := NewClient("localhost", "443")
	cli.httpClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		contexts <- request.Context()

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("{}")),
			Request:    request,
		}, nil
	})

	return cli, contexts
}

func TestNewClient_Timeouts(t *testing.T) {
	cli := NewClient("localhost", "443")

	// requests have no overall time limit, so file downloads aren't cut off
	assert.Equal(t, cli.httpClient.Timeout, time.Duration(0))

	// but a server which doesn't respond at all doesn't block a request forever
	transport := cli.httpClient.Transport.(*http.Transport)
	assert.Equal(t, transport.ResponseHeaderTimeout, apiCallTimeout)
}

func TestClient_Make_LimitsAPICalls(t *testing.T) {
	cli, contexts := newRecordingClient()

	start := time.Now()
	assert.NoError(t, cli.Get(t.Context(), "/v1/test", nil))
	end := time.Now()

	// the request, including reading the response body, is limited by apiCallTimeout
	deadline, hasDeadline := (<-contexts).Deadline()
	assert.True(t, hasDeadline)
	assert.False(t, deadline.Before(start.Add(apiCallTimeout)))
	assert.False(t, deadline.After(end.Add(apiCallTimeout)))
}

func TestClient_Make_KeepsEarlierCallerDeadline(t *testing.T) {
	cli, contexts := newRecordingClient()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	callerDeadline, _ := ctx.Deadline()

	assert.NoError(t, cli.Get(ctx, "/v1/test", nil))

	deadline, _ := (<-contexts).Deadline()
	assert.Equal(t, deadline, callerDeadline)
}

func TestClient_Do_HasNoTimeLimitOfItsOwn(t *testing.T) {
	cli, contexts := newRecordingClient()

	// file downloads use Do directly, so they can take as long as the caller allows
	request, err := cli.NewRequest(t.Context(), http.MethodGet, "/v1/file", nil)
	assert.NoError(t, err)

	response, err := cli.Do(request)
	assert.NoError(t, err)
	_ = response.Body.Close()

	_, hasDeadline := (<-contexts).Deadline()
	assert.False(t, hasDeadline)

	// a download during configuration execution is limited by the execution's time limit
	ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
	defer cancel()
	executionDeadline, _ := ctx.Deadline()

	request, err = cli.NewRequest(ctx, http.MethodGet, "/v1/file", nil)
	assert.NoError(t, err)

	response, err = cli.Do(request)
	assert.NoError(t, err)
	_ = response.Body.Close()

	deadline, _ := (<-contexts).Deadline()
	assert.Equal(t, deadline, executionDeadline)
}
