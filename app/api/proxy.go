package api

import (
	"fmt"
	"os"
)

const proxyEnvVar = "HTTP_PROXY"

// UseProxy sets HTTP_PROXY environmental variable, so HTTP clients can make use of it.
func UseProxy(host, port, user, password string) error {
	// if proxy server is not specified or proxy is already set in the environment, return nil.
	if host == "" || os.Getenv(proxyEnvVar) != "" {
		return nil
	}

	proxyURL := fmt.Sprintf("%s:%s", host, port)

	if user != "" {
		proxyURL = fmt.Sprintf("%s:%s@%s", user, password, proxyURL)
	}

	proxyURL = "http://" + proxyURL

	if err := os.Setenv(proxyEnvVar, proxyURL); err != nil {
		return fmt.Errorf("error setting up HTTP proxy: %w", err)
	}

	return nil
}
