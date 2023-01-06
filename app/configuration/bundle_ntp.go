package configuration

// NTP configures NTP servers.
//
// Example payload:
// {
//   "servers": [
//    "pool1.ntp.org"
//  ]
// }
type NTP struct {
	Metadata

	Servers []string `json:"servers"`
}
