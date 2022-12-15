package agent

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
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

	proxyURL := fmt.Sprintf("%s:%d", agent.cfg.ProxyServer, agent.cfg.ProxyPort)

	if agent.cfg.ProxyUser != "" {
		proxyURL = fmt.Sprintf("%s:%s@%s", agent.cfg.ProxyUser, agent.cfg.ProxyPassword, proxyURL)
	}

	proxyURL = "http://" + proxyURL

	if err := os.Setenv(proxyEnvVar, proxyURL); err != nil {
		return fmt.Errorf("error setting up HTTP proxy: %w", err)
	}

	return nil
}

// PublicHTTPClient returns a public HTTPClient with no client authentication.
func (agent *Agent) PublicHTTPClient() *http.Client {
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
				RootCAs:            agent.rootCAPool,
				InsecureSkipVerify: os.Getenv("INSECURE") == "1",
			},
		},
		Timeout: 60 * time.Second,
	}
}
