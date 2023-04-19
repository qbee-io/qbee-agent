package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// MockResponse represents a mock response with the request that was used to get it.
type MockResponse struct {
	called       bool
	httpRequest  *http.Request
	httpResponse *http.Response
}

// Called returns true if the response was used.
func (resp *MockResponse) Called() bool {
	return resp.called
}

// Request returns the request that was used to get this response.
func (resp *MockResponse) Request() *http.Request {
	return resp.httpRequest
}

// Mock is a mock RoundTripper implementation.
type Mock struct {
	mockResponses []*MockResponse
}

// AddResponse adds a new mock response.
func (m *Mock) AddResponse(response *http.Response) *MockResponse {
	mockResponse := &MockResponse{
		httpResponse: response,
	}

	m.mockResponses = append(m.mockResponses, mockResponse)

	return mockResponse
}

// Add adds a new mock response with the given status code and body.
// Response body can contain format specifiers.
func (m *Mock) Add(statusCode int, body string, args ...any) *MockResponse {
	return m.AddResponse(&http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(fmt.Sprintf(body, args...))),
	})
}

// RoundTrip is the RoundTripper interface implementation, so we can use this in http.Client.
func (m *Mock) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(m.mockResponses) == 0 {
		return nil, fmt.Errorf("no more mockResponses")
	}

	response := m.mockResponses[0]
	m.mockResponses = m.mockResponses[1:]

	response.called = true
	response.httpRequest = req

	return response.httpResponse, nil
}

// NewMockedClient returns a new API client with mocked transport.
func NewMockedClient() (*Client, *Mock) {
	mock := &Mock{}
	cli := &Client{httpClient: &http.Client{Transport: mock}}

	return cli, mock
}
