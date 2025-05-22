//go:build unix

package agent

import "fmt"

func RunService(cfg *Config) error {
	return fmt.Errorf("service is not supported on this platform")
}
