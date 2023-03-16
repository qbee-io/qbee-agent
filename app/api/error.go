package api

import (
	"fmt"
	"io"
)

// ConnectionError is used to explicitly indicate API connectivity issue.
// This is used to track failed API connection attempts.
type ConnectionError error

// Error returned when HTTP API request results in status code >= 400.
type Error struct {
	ResponseCode int
	ResponseBody []byte
}

func (err *Error) Error() string {
	return fmt.Sprintf("unexpected API response: %d %s", err.ResponseCode, err.ResponseBody)
}

func NewError(responseStatusCode int, responseBody io.Reader) error {
	responseBodyContents, _ := io.ReadAll(responseBody)

	return &Error{
		ResponseCode: responseStatusCode,
		ResponseBody: responseBodyContents,
	}
}
