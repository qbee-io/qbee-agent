package remoteaccess

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"qbee.io/platform/test/assert"

	"github.com/qbee-io/qbee-agent/app/api"
)

func TestService_downloadOpenVPN(t *testing.T) {
	certDir := t.TempDir()
	binDir := t.TempDir()
	apiClient, apiMock := api.NewMockedClient()

	mock1 := apiMock.AddResponse(&http.Response{
		StatusCode: http.StatusOK,
		Header: map[string][]string{
			"X-Binary-Version": {"1.0.0"},
			"X-Binary-Digest": {
				"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
			},
			"X-Binary-Signature": {
				"MEYCIQCbbgslVegJXFczWSLP0lFKflbXdOtgMWslm/AQy1nIRQIhALjSztLgg4JltImIy33adWkH3WHS3+5F/aI1jk5/KrB8",
			},
		},
		Body: io.NopCloser(bytes.NewBufferString("test")),
	})

	service := New(apiClient, "127.0.0.1", certDir, binDir, nil)

	ctx := context.Background()

	if err := service.downloadOpenVPN(ctx); err != nil {
		t.Errorf("downloadOpenVPN() error = %v", err)
	}

	expectedBinaryPath := filepath.Join(binDir, "openvpn")

	if data, err := os.ReadFile(expectedBinaryPath); err != nil {
		t.Errorf("cannot read binary file = %v", err)
	} else if !bytes.Equal(data, []byte("test")) {
		t.Errorf("binary file content mismatch 'test' != %v", data)
	}

	stats, err := os.Stat(expectedBinaryPath)
	if err != nil {
		t.Errorf("cannot read binary file stats: %v", err)
	}

	if stats.Mode() != 0700 {
		t.Errorf("binary file not executable: %v", stats.Mode())
	}

	assert.True(t, mock1.Called())
	assert.Equal(t, mock1.Request().Method, http.MethodGet)
	assert.Equal(t, mock1.Request().URL.RequestURI(), "/v1/org/device/auth/download/openvpn/"+runtime.GOARCH)
}

func TestService_downloadOpenVPN_alreadyExists(t *testing.T) {
	certDir := t.TempDir()
	binDir := t.TempDir()
	apiClient, _ := api.NewMockedClient()
	expectedBinaryPath := filepath.Join(binDir, "openvpn")

	if err := os.WriteFile(expectedBinaryPath, []byte("test"), 0700); err != nil {
		t.Errorf("cannot write binary file = %v", err)
	}

	service := New(apiClient, "127.0.0.1", certDir, binDir, nil)

	ctx := context.Background()

	// Should not download the binary if it already exists.
	if err := service.downloadOpenVPN(ctx); err != nil {
		t.Errorf("downloadOpenVPN() error = %v", err)
	}
}

func TestService_refreshCredentials(t *testing.T) {
	certDir := t.TempDir()
	binDir := t.TempDir()
	apiClient, apiMock := api.NewMockedClient()

	mock1 := apiMock.Add(http.StatusOK, `{
		"vpn_ca_cert": ["line1", "line2"],
		"vpn_cert": ["line3", "line4"],
		"vpn_cert_expiry": 1234567890,
		"status": "OK"
	}`)

	service := New(apiClient, "127.0.0.1", certDir, binDir, nil)

	ctx := context.Background()

	if err := service.refreshCredentials(ctx); err != nil {
		t.Errorf("refreshCredentials() error = %v", err)
	}

	// Make sure that the right API was called.
	assert.True(t, mock1.Called())
	assert.Equal(t, mock1.Request().Method, http.MethodGet)
	assert.Equal(t, mock1.Request().URL.RequestURI(), "/v1/org/device/auth/vpncert")

	// check that the files were written to disk with correct mode
	expectedFiles := []string{
		filepath.Join(certDir, "qbee-ca-vpn.cert"),
		filepath.Join(certDir, "qbee-vpn.cert"),
	}

	expectedFileContents := []string{
		"line1\nline2",
		"line3\nline4",
	}

	for i := range expectedFiles {
		filePath := expectedFiles[i]
		fileContents := expectedFileContents[i]

		if data, err := os.ReadFile(filePath); err != nil {
			t.Errorf("cannot read %s file = %v", filePath, err)
		} else if !bytes.Equal(data, []byte(fileContents)) {
			t.Errorf("%s file content mismatch '%s' != '%s'", filePath, data, fileContents)
		}

		if stats, err := os.Stat(filePath); err != nil {
			t.Errorf("cannot read %s file stats: %v", filePath, err)
		} else if stats.Mode() != 0600 {
			t.Errorf("%s mode error: %v != %v", filePath, stats.Mode(), 0600)
		}
	}

	// Check that the credentials were saved to the service.
	assert.Equal(t, service.credentials.CACertificatePEM(), []byte("line1\nline2"))
	assert.Equal(t, service.credentials.CertificatePEM(), []byte("line3\nline4"))
	assert.Equal(t, service.credentials.Expiry, int64(1234567890))
}

func TestService_refreshCredentials_expiring(t *testing.T) {
	certDir := t.TempDir()
	binDir := t.TempDir()
	apiClient, apiMock := api.NewMockedClient()

	newExpiry := time.Now().Add(time.Hour).Unix()

	mock1 := apiMock.Add(http.StatusOK, `{
		"vpn_ca_cert": ["line1", "line2"],
		"vpn_cert": ["line3", "line4"],
		"vpn_cert_expiry": %d,
		"status": "OK"
	}`, newExpiry)

	service := New(apiClient, "127.0.0.1", certDir, binDir, nil)

	// Set the credentials to expire in 14 minutes.
	service.credentials.Expiry = time.Now().Add(14 * time.Minute).Unix()

	ctx := context.Background()

	if err := service.refreshCredentials(ctx); err != nil {
		t.Errorf("refreshCredentials() error = %v", err)
	}

	assert.True(t, mock1.Called())
	assert.Equal(t, service.credentials.Expiry, newExpiry)

	// Call again, should not refresh the credentials.
	// API mock will fail if a new request is made,
	// so this call will return an error if it tries to refresh the credentials.
	if err := service.refreshCredentials(ctx); err != nil {
		t.Errorf("refreshCredentials() error = %v", err)
	}
}

func TestService_ensureRunning(t *testing.T) {
	certDir := t.TempDir()
	binDir := t.TempDir()
	apiClient, apiMock := api.NewMockedClient()

	openVPNPath := filepath.Join(binDir, "openvpn")
	testFilePath := filepath.Join(binDir, "test")
	openVPNBinary := []byte(fmt.Sprintf(`#!/bin/sh
		echo -n "$@" > %s 
	`, testFilePath))

	const executableFileMode = 0700

	if err := os.WriteFile(openVPNPath, openVPNBinary, executableFileMode); err != nil {
		t.Errorf("cannot write binary file = %v", err)
	}

	credentialsAPIMock := apiMock.Add(http.StatusOK, `{"status": "OK"}`)

	service := New(apiClient, "127.0.0.1", certDir, binDir, nil)

	service.enabled = true

	service.ensureRunning()
	service.activeProcesses.Wait()

	// make sure that the credentials were refreshed.
	assert.True(t, credentialsAPIMock.Called())
	assert.Equal(t, credentialsAPIMock.Request().Method, http.MethodGet)
	assert.Equal(t, credentialsAPIMock.Request().URL.RequestURI(), "/v1/org/device/auth/vpncert")

	// Check that the openvpn binary was called with the right parameters.
	data, err := os.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("cannot read %s file = %v", testFilePath, err)
	}

	arguments := strings.Fields(string(data))
	expectedArguments := []string{
		"--client",
		"--remote", "127.0.0.1",
		"--dev", "qbee0",
		"--dev-type", "tun",
		"--proto", "tcp",
		"--port", "443",
		"--nobind",
		"--auth-nocache",
		"--script-security", "1",
		"--persist-key",
		"--persist-tun",
		"--ca", filepath.Join(certDir, "qbee-ca-vpn.cert"),
		"--cert", filepath.Join(certDir, "qbee-vpn.cert"),
		"--key", filepath.Join(certDir, "qbee.key"),
		"--verb", "0",
		"--suppress-timestamps",
		"--remote-cert-tls", "server",
		"--disable-occ",
		"--cipher", "AES-256-GCM",
	}
	assert.Equal(t, arguments, expectedArguments)
}

func TestService_start_checkStatus_stop(t *testing.T) {
	certDir := t.TempDir()
	binDir := t.TempDir()
	apiClient, _ := api.NewMockedClient()

	openVPNPath := filepath.Join(binDir, "openvpn")
	openVPNBinary := []byte(`#!/bin/sh
		sleep 10 
	`)

	const executableFileMode = 0700

	if err := os.WriteFile(openVPNPath, openVPNBinary, executableFileMode); err != nil {
		t.Errorf("cannot write binary file = %v", err)
	}

	service := New(apiClient, "127.0.0.1", certDir, binDir, nil)
	service.credentials.Expiry = time.Now().Add(time.Hour).Unix()

	ctx := context.Background()

	if service.checkStatus() {
		t.Errorf("checkStatus() expected false")
	}

	if err := service.UpdateState(ctx, true); err != nil {
		t.Errorf("UpdateState() error = %v", err)
	}

	if !service.checkStatus() {
		t.Errorf("checkStatus() expected true")
	}

	stopTime := time.Now()
	if err := service.UpdateState(ctx, false); err != nil {
		t.Errorf("UpdateState() error = %v", err)
	}

	service.activeProcesses.Wait()

	if time.Since(stopTime) > stopTimeout+time.Second {
		t.Errorf("UpdateState(false) expected to stop the test process immediately")
	}

	if service.checkStatus() {
		t.Errorf("checkStatus() expected false")
	}
}
