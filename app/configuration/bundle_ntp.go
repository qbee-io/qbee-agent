package configuration

import (
	"context"
	"fmt"
)

// NTPBundle configures NTP servers.
//
// Example payload:
// {
//   "servers": [
//    "pool1.ntp.org"
//  ]
// }
type NTPBundle struct {
	Metadata

	Servers []string `json:"servers"`
}

// Execute returns a deprecation error message.
// We don't want to support NTP as its own bundle, since it can be configured using software management.
func (ntp NTPBundle) Execute(_ context.Context, _ *Service) error {
	return fmt.Errorf("NTP is no longer supported - please use software management to configure NTP")
}
