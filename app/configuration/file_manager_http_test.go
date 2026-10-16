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
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/utils/assert"
)

// newMockedFileService returns a Service whose API calls are answered by the returned api.Mock.
// Any request without a queued mock response fails.
func newMockedFileService(t *testing.T) (*Service, *api.Mock) {
	apiClient, mock := api.NewMockedClient()

	return New(apiClient, t.TempDir(), t.TempDir()), mock
}

func fileResponse(statusCode int, body io.Reader) *http.Response {
	return &http.Response{StatusCode: statusCode, Body: io.NopCloser(body)}
}

// serveFile emulates the file manager download endpoint, honoring "bytes=<offset>-" Range requests.
func serveFile(contents []byte) api.MockHandlerFunc {
	return func(req *http.Request) (*http.Response, error) {
		rangeHeader := req.Header.Get("Range")
		if rangeHeader == "" {
			return fileResponse(http.StatusOK, bytes.NewReader(contents)), nil
		}

		var offset int
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &offset); err != nil || offset >= len(contents) {
			return fileResponse(http.StatusRequestedRangeNotSatisfiable, http.NoBody), nil
		}

		return fileResponse(http.StatusPartialContent, bytes.NewReader(contents[offset:])), nil
	}
}

// serveFileInterrupted behaves like serveFile, but the connection breaks after sending n body bytes.
func serveFileInterrupted(contents []byte, n int) api.MockHandlerFunc {
	return func(req *http.Request) (*http.Response, error) {
		resp, err := serveFile(contents)(req)
		if err != nil {
			return nil, err
		}

		resp.Body = io.NopCloser(io.MultiReader(io.LimitReader(resp.Body, int64(n)), iotest.ErrReader(io.ErrUnexpectedEOF)))

		return resp, nil
	}
}

func assertFileRequest(t *testing.T, resp *api.MockResponse, path, rangeHeader string) {
	t.Helper()

	assert.True(t, resp.Called())
	assert.Equal(t, resp.Request().URL.Path, path)
	assert.Equal(t, resp.Request().Header.Get("Range"), rangeHeader)
}

func testFileContents() []byte {
	return bytes.Repeat([]byte("0123456789abcdefghijklmnopqrstuvwxyz\n"), 4096)
}

func testFileMetadata(contents []byte) *FileMetadata {
	return &FileMetadata{
		Tags: map[string]string{fileDigestSHA256Tag: sha256Hex(contents)},
		Size: int64(len(contents)),
	}
}

func writePartialDownload(t *testing.T, dst string, fileMetadata *FileMetadata, contents []byte) string {
	t.Helper()

	partialPath := GetPartialDownloadFilePath(dst, fileMetadata.SHA256())
	assert.NoError(t, os.Mkdir(filepath.Dir(partialPath), 0700))
	assert.NoError(t, os.WriteFile(partialPath, contents, 0600))

	return partialPath
}

func assertNoPartialDownload(t *testing.T, dst string, fileMetadata *FileMetadata) {
	t.Helper()

	partialPath := GetPartialDownloadFilePath(dst, fileMetadata.SHA256())

	_, err := os.Stat(filepath.Dir(partialPath))
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

func assertFileContents(t *testing.T, path string, contents []byte) {
	t.Helper()

	got, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, got, contents)
}

func assertNotExist(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

const testFileAPIPath = "/v1/org/device/auth/files/file.txt"

func Test_downloadHTTP_FullDownload(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)
	resp := mock.AddHandler(serveFile(contents))

	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)

	created, err := srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)
	assertFileContents(t, dst, contents)
	assertFileRequest(t, resp, testFileAPIPath, "")
	assertNoPartialDownload(t, dst, fileMetadata)

	// no response is queued, so a second request would fail
	created, err = srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	assert.NoError(t, err)
	assert.False(t, created)
}

func Test_downloadHTTP_MetadataFromAPI(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)
	metadataResp := mock.Add(http.StatusOK, `{"status":"ok","data":{"tags":{"%s":"%s"},"size":%d}}`,
		fileDigestSHA256Tag, sha256Hex(contents), len(contents))
	fileResp := mock.AddHandler(serveFile(contents))

	dst := filepath.Join(t.TempDir(), "file.txt")

	created, err := srv.downloadFile(t.Context(), "", "dir/file.txt", dst, File{})
	assert.NoError(t, err)
	assert.True(t, created)
	assertFileContents(t, dst, contents)

	assert.True(t, metadataResp.Called())
	assert.Equal(t, metadataResp.Request().URL.Path, "/v1/org/device/auth/filemetadata/dir/file.txt")
	assertFileRequest(t, fileResp, "/v1/org/device/auth/files/dir/file.txt", "")
}

func Test_downloadHTTP_ResumesPartialDownload(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)
	resp := mock.AddHandler(serveFile(contents))

	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)

	offset := len(contents) / 3
	writePartialDownload(t, dst, fileMetadata, contents[:offset])

	created, err := srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)
	assertFileContents(t, dst, contents)
	assertFileRequest(t, resp, testFileAPIPath, fmt.Sprintf("bytes=%d-", offset))
	assertNoPartialDownload(t, dst, fileMetadata)
}

func Test_downloadHTTP_ResumesAfterInterruptedDownload(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)

	interruptAfter := len(contents) / 2
	interruptedResp := mock.AddHandler(serveFileInterrupted(contents, interruptAfter))
	resumedResp := mock.AddHandler(serveFile(contents))

	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)
	partialPath := GetPartialDownloadFilePath(dst, fileMetadata.SHA256())

	created, err := srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected interrupted download to fail with unexpected EOF, got %v", err)
	}
	assert.False(t, created)
	assertNotExist(t, dst)
	assertFileRequest(t, interruptedResp, testFileAPIPath, "")

	// the bytes received before the interruption are kept for resuming
	assertFileContents(t, partialPath, contents[:interruptAfter])

	created, err = srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)
	assertFileContents(t, dst, contents)
	assertFileRequest(t, resumedResp, testFileAPIPath, fmt.Sprintf("bytes=%d-", interruptAfter))
	assertNoPartialDownload(t, dst, fileMetadata)
}

func Test_downloadHTTP_CompletePartialIsNotRequestedAgain(t *testing.T) {
	contents := testFileContents()
	// no response is queued, so a request (which would get 416 from a real server) fails the download
	srv, _ := newMockedFileService(t)

	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)
	writePartialDownload(t, dst, fileMetadata, contents)

	created, err := srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)
	assertFileContents(t, dst, contents)
	assertNoPartialDownload(t, dst, fileMetadata)
}

func Test_downloadHTTP_EmptyFile(t *testing.T) {
	srv, _ := newMockedFileService(t)

	dst := filepath.Join(t.TempDir(), "empty.txt")
	fileMetadata := testFileMetadata([]byte{})

	created, err := srv.downloadMetadataCompare(t.Context(), "", "empty.txt", dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)
	assertFileContents(t, dst, []byte{})
}

func Test_downloadHTTP_OversizedPartialIsRestarted(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)
	resp := mock.AddHandler(serveFile(contents))

	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)
	writePartialDownload(t, dst, fileMetadata, append(bytes.Clone(contents), []byte("garbage")...))

	created, err := srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)
	assertFileContents(t, dst, contents)
	assertFileRequest(t, resp, testFileAPIPath, "")
	assertNoPartialDownload(t, dst, fileMetadata)
}

func Test_downloadHTTP_CorruptedPartialIsDiscarded(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)
	resumedResp := mock.AddHandler(serveFile(contents))
	restartedResp := mock.AddHandler(serveFile(contents))

	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)

	offset := len(contents) / 2
	writePartialDownload(t, dst, fileMetadata, bytes.Repeat([]byte("x"), offset))

	created, err := srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	if err == nil || !strings.Contains(err.Error(), "invalid contents") {
		t.Fatalf("expected invalid contents error, got %v", err)
	}
	assert.False(t, created)
	assertNotExist(t, dst)
	assertFileRequest(t, resumedResp, testFileAPIPath, fmt.Sprintf("bytes=%d-", offset))
	assertNoPartialDownload(t, dst, fileMetadata)

	created, err = srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	assert.NoError(t, err)
	assert.True(t, created)
	assertFileContents(t, dst, contents)
	assertFileRequest(t, restartedResp, testFileAPIPath, "")
}

func Test_downloadHTTP_ServerIgnoringRangeIsRejected(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)
	resp := mock.AddResponse(fileResponse(http.StatusOK, bytes.NewReader(contents)))

	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)

	offset := len(contents) / 4
	partialPath := writePartialDownload(t, dst, fileMetadata, contents[:offset])

	created, err := srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("unexpected status code: %d", http.StatusOK)) {
		t.Fatalf("expected unexpected status code error, got %v", err)
	}
	assert.False(t, created)
	assertFileRequest(t, resp, testFileAPIPath, fmt.Sprintf("bytes=%d-", offset))

	// a full body must not be appended to the partial download
	assertFileContents(t, partialPath, contents[:offset])
	assertNotExist(t, dst)
}

func Test_downloadHTTP_UnexpectedStatus(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		partialLength int
	}{
		{name: "not found", status: http.StatusNotFound},
		{name: "server error", status: http.StatusInternalServerError},
		{name: "partial content without range", status: http.StatusPartialContent},
		{name: "range not satisfiable", status: http.StatusRequestedRangeNotSatisfiable, partialLength: 10},
		{name: "server error on resume", status: http.StatusInternalServerError, partialLength: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents := testFileContents()
			srv, mock := newMockedFileService(t)
			resp := mock.Add(tt.status, "")

			dst := filepath.Join(t.TempDir(), "file.txt")
			fileMetadata := testFileMetadata(contents)

			partialPath := GetPartialDownloadFilePath(dst, fileMetadata.SHA256())
			if tt.partialLength > 0 {
				writePartialDownload(t, dst, fileMetadata, contents[:tt.partialLength])
			}

			created, err := srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("unexpected status code: %d", tt.status)) {
				t.Fatalf("expected unexpected status code error, got %v", err)
			}
			assert.False(t, created)
			assert.True(t, resp.Called())
			assertNotExist(t, dst)
			assertFileContents(t, partialPath, contents[:tt.partialLength])
		})
	}
}

func Test_downloadHTTP_SourceShorterThanMetadata(t *testing.T) {
	contents := testFileContents()
	srv, mock := newMockedFileService(t)
	mock.AddHandler(serveFile(contents[:len(contents)/2]))

	dst := filepath.Join(t.TempDir(), "file.txt")
	fileMetadata := testFileMetadata(contents)

	created, err := srv.downloadMetadataCompare(t.Context(), "", "file.txt", dst, fileMetadata)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF error for truncated source, got %v", err)
	}
	assert.False(t, created)
	assertNotExist(t, dst)
}
