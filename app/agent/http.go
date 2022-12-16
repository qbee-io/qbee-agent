package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const proxyEnvVar = "HTTP_PROXY"

const UserAgent = "qbee-agent/" + Version

// useProxy sets HTTP_PROXY environmental variable, so HTTP clients can make use of it.
func (agent *Agent) useProxy() error {
	// if proxy server is not specified or proxy is already set in the environment, return.
	if agent.cfg.ProxyServer == "" || os.Getenv(proxyEnvVar) != "" {
		return nil
	}

	proxyURL := fmt.Sprintf("%s:%s", agent.cfg.ProxyServer, agent.cfg.ProxyPort)

	if agent.cfg.ProxyUser != "" {
		proxyURL = fmt.Sprintf("%s:%s@%s", agent.cfg.ProxyUser, agent.cfg.ProxyPassword, proxyURL)
	}

	proxyURL = "http://" + proxyURL

	if err := os.Setenv(proxyEnvVar, proxyURL); err != nil {
		return fmt.Errorf("error setting up HTTP proxy: %w", err)
	}

	return nil
}

// anonymousHTTPClient returns an authenticatedHTTPClient without client TLS certificates.
func (agent *Agent) anonymousHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   60 * time.Second,
				KeepAlive: 60 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          5,
			IdleConnTimeout:       60 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig: &tls.Config{
				RootCAs: agent.rootCAPool,
			},
		},
		Timeout: 60 * time.Second,
	}
}

// authenticatedHTTPClient returns an authenticatedHTTPClient with client TLS certificate.
func (agent *Agent) authenticatedHTTPClient() *http.Client {
	if agent.httpClient != nil {
		return agent.httpClient
	}

	agent.httpClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   60 * time.Second,
				KeepAlive: 60 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          5,
			IdleConnTimeout:       60 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig: &tls.Config{
				RootCAs: agent.rootCAPool,
				Certificates: []tls.Certificate{{
					Certificate: [][]byte{agent.certificate.Raw},
					PrivateKey:  agent.privateKey,
				}},
			},
		},
		Timeout: 60 * time.Second,
	}

	return agent.httpClient
}

// apiRequest sends an HTTP request to the device hub API with device's identity.
func (agent *Agent) apiRequest(ctx context.Context, method, path string, payload any) (*http.Response, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("path %s must start with /", path)
	}

	url := fmt.Sprintf("https://%s:%s%s", agent.cfg.DeviceHubServer, agent.cfg.DeviceHubPort, path)

	// check if payload is a bytes.Buffer pointer, then we can use it as-is
	body, alreadyBytesBuffer := payload.(*bytes.Buffer)
	if payload != nil && !alreadyBytesBuffer {
		body = new(bytes.Buffer)

		if err := json.NewEncoder(body).Encode(payload); err != nil {
			return nil, fmt.Errorf("error marshaling request body for %s %s: %w", method, path, err)
		}
	}

	request, err := http.NewRequestWithContext(ctx, method, url, compressRequestBody(body))
	if err != nil {
		return nil, fmt.Errorf("error initializing http request %s %s: %w", method, path, err)
	}

	if body != nil {
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Content-Encoding", "gzip")
	}

	request.Header.Set("User-Agent", UserAgent)

	var response *http.Response
	if response, err = agent.authenticatedHTTPClient().Do(request); err != nil {
		return nil, fmt.Errorf("error sending request %s %s: %w", method, path, err)
	}

	return response, nil
}

// compressRequestBody returns io.Reader with compressed body payload
func compressRequestBody(body *bytes.Buffer) io.Reader {
	if body == nil {
		return nil
	}

	compressedBuffer := new(bytes.Buffer)

	gzipWriter := gzip.NewWriter(compressedBuffer)
	if _, err := gzipWriter.Write(body.Bytes()); err != nil {
		panic(err) // since we are operating on bytes.Buffer, this shouldn't ever error out
	}

	if err := gzipWriter.Close(); err != nil {
		panic(err) // since we are operating on bytes.Buffer, this shouldn't ever error out
	}

	return compressedBuffer
}
