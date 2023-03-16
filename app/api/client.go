package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/qbee-io/qbee-agent/app"
)

const UserAgent = "qbee-agent/" + app.Version

// apiCallTimeout defines total request/response time we allow for any API call.
// This timeout doesn't apply to file downloads.
const apiCallTimeout = 10 * time.Second

type Client struct {
	host       string
	port       string
	httpClient *http.Client
}

// NewClient returns a new device hub client.
func NewClient(host, port string, rootCAPool *x509.CertPool) *Client {
	return &Client{
		host: host,
		port: port,
		httpClient: &http.Client{
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
					RootCAs: rootCAPool,
				},
			},
			Timeout: 60 * time.Second,
		},
	}
}

// UseTLSCredentials adds TLS certificate to TLSClientConfig.
func (cli *Client) UseTLSCredentials(privateKey *ecdsa.PrivateKey, certificate *x509.Certificate) {
	cli.httpClient.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{
		{
			Certificate: [][]byte{certificate.Raw},
			PrivateKey:  privateKey,
		},
	}
}

// NewRequest returns a new HTTP request for provided method, path and src.
func (cli *Client) NewRequest(ctx context.Context, method, path string, src any) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("path %s must start with /", path)
	}

	// check if payload is a bytes.Buffer pointer, then we can use it as-is
	body, alreadyBytesBuffer := src.(*bytes.Buffer)
	if src != nil && !alreadyBytesBuffer {
		body = new(bytes.Buffer)

		if err := json.NewEncoder(body).Encode(src); err != nil {
			return nil, fmt.Errorf("error marshaling request body for %s %s: %w", method, path, err)
		}
	}

	url := fmt.Sprintf("https://%s:%s%s", cli.host, cli.port, path)

	request, err := http.NewRequestWithContext(ctx, method, url, compressRequestBody(body))
	if err != nil {
		return nil, fmt.Errorf("error initializing http request %s %s: %w", method, path, err)
	}

	if body != nil {
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Content-Encoding", "gzip")
	}

	return request, nil
}

// Make sends an API request and optionally parses response body into dst.
func (cli *Client) Make(request *http.Request, dst any) error {
	response, err := cli.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return NewError(response.StatusCode, response.Body)
	}

	if dst != nil {
		if err = json.NewDecoder(response.Body).Decode(dst); err != nil {
			return fmt.Errorf("cannot decode API response body: %w", err)
		}
	}

	return nil
}

// Do sends an HTTP request and returns an HTTP response.
func (cli *Client) Do(request *http.Request) (*http.Response, error) {
	request.Header.Set("User-Agent", UserAgent)
	request.Header.Set("Cache-Control", "no-cache")

	response, err := cli.httpClient.Do(request)
	if err != nil {
		return nil, ConnectionError(fmt.Errorf("failed to send API request: %w", err))
	}

	return response, nil
}

// request creates, sends and processes response for an HTTP request.
func (cli *Client) request(ctx context.Context, method, path string, src, dst any) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, apiCallTimeout)
	defer cancel()

	request, err := cli.NewRequest(ctxWithTimeout, method, path, src)
	if err != nil {
		return err
	}

	return cli.Make(request, dst)
}

// Get sends a GET request to device hub.
func (cli *Client) Get(ctx context.Context, path string, dst any) error {
	return cli.request(ctx, http.MethodGet, path, nil, dst)
}

// Post sends a POST request to device hub.
func (cli *Client) Post(ctx context.Context, path string, src, dst any) error {
	return cli.request(ctx, http.MethodPost, path, src, dst)
}

// Put sends a PUT request to device hub.
func (cli *Client) Put(ctx context.Context, path string, src, dst any) error {
	return cli.request(ctx, http.MethodPut, path, src, dst)
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
