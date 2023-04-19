package api

import (
	"fmt"
	"os"
)

const proxyEnvVar = "HTTP_PROXY"

// Proxy represents a proxy server configuration.
type Proxy struct {
	Host     string
	Port     string
	User     string
	Password string
}

// UseProxy sets HTTP_PROXY environmental variable, so HTTP clients can make use of it.
func UseProxy(proxy *Proxy) error {
	// if proxy server is not specified or proxy is already set in the environment, return nil.
	if proxy == nil || os.Getenv(proxyEnvVar) != "" {
		return nil
	}

	proxyURL := fmt.Sprintf("%s:%s", proxy.Host, proxy.Port)

	if proxy.User != "" {
		proxyURL = fmt.Sprintf("%s:%s@%s", proxy.User, proxy.Password, proxyURL)
	}

	proxyURL = "http://" + proxyURL

	if err := os.Setenv(proxyEnvVar, proxyURL); err != nil {
		return fmt.Errorf("error setting up HTTP proxy: %w", err)
	}

	return nil
}
