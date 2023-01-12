package configuration

import (
	"context"
	"fmt"
	"strconv"
)

// ConnectivityWatchdogBundle configures a watchdog.
//
// Example payload:
// {
//   "threshold": "3"
// }
type ConnectivityWatchdogBundle struct {
	Metadata

	Threshold string `json:"threshold"`
}

// Execute connectivity watchdog configuration bundle.
func (c ConnectivityWatchdogBundle) Execute(_ context.Context, service *Service) error {
	threshold, err := strconv.Atoi(c.Threshold)
	if err != nil {
		return fmt.Errorf("invalid threshold value")
	}

	service.connectivityWatchdogThreshold = threshold

	return nil
}
