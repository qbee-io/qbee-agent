package api

import (
	"fmt"
	"io"
)

// ConnectionError is used to explicitly indicate API connectivity issue.
// This is used to track failed API connection attempts.
type ConnectionError error

// HTTPError returned when HTTP request results in status code >= 400.
type HTTPError struct {
	ResponseCode int
	ResponseBody []byte
}

func (err *HTTPError) Error() string {
	return fmt.Sprintf("unexpected HTTP response: %d %s", err.ResponseCode, err.ResponseBody)
}

func NewHTTPError(responseStatusCode int, responseBody io.Reader) error {
	responseBodyContents, _ := io.ReadAll(responseBody)

	return &HTTPError{
		ResponseCode: responseStatusCode,
		ResponseBody: responseBodyContents,
	}
}
