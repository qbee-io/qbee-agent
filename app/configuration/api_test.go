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

package configuration

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.qbee.io/agent/app/api"
)

func newTestServiceWithServer(t *testing.T, handler http.HandlerFunc) (*Service, *httptest.Server) {
	t.Helper()

	ts := httptest.NewTLSServer(handler)
	t.Cleanup(ts.Close)

	apiClient := api.NewClient("localhost", "0")
	apiClient.WithTLSConfig(&tls.Config{InsecureSkipVerify: true})
	apiClient.WithPort(fmt.Sprintf("%d", ts.Listener.Addr().(*net.TCPAddr).Port))

	srv := New(apiClient, t.TempDir(), "")

	return srv, ts
}

func Test_getFileFromAPI(t *testing.T) {
	t.Run("200 OK for a full download", func(t *testing.T) {
		srv, _ := newTestServiceWithServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Range") != "" {
				t.Fatalf("unexpected Range header: %s", r.Header.Get("Range"))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("file-contents"))
		})

		body, err := srv.getFileFromAPI(t.Context(), "some/file", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = body.Close() }()

		got, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("unexpected error reading body: %v", err)
		}
		if string(got) != "file-contents" {
			t.Fatalf("unexpected body: %s", got)
		}
	})

	t.Run("206 Partial Content for a ranged download", func(t *testing.T) {
		srv, _ := newTestServiceWithServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Range") != "bytes=10-" {
				t.Fatalf("unexpected Range header: %s", r.Header.Get("Range"))
			}
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("rest-of-file"))
		})

		body, err := srv.getFileFromAPI(t.Context(), "some/file", 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = body.Close() }()

		got, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("unexpected error reading body: %v", err)
		}
		if string(got) != "rest-of-file" {
			t.Fatalf("unexpected body: %s", got)
		}
	})

	t.Run("error when range request does not return 206", func(t *testing.T) {
		srv, _ := newTestServiceWithServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("whole-file-again"))
		})

		_, err := srv.getFileFromAPI(t.Context(), "some/file", 10)
		if err == nil {
			t.Fatalf("expected error for unexpected status code, got nil")
		}
	})

	t.Run("error when full request does not return 200", func(t *testing.T) {
		srv, _ := newTestServiceWithServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := srv.getFileFromAPI(t.Context(), "some/file", 0)
		if err == nil {
			t.Fatalf("expected error for unexpected status code, got nil")
		}
	})
}
