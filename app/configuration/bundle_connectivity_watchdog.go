package configuration

// ConnectivityWatchdog configures a watchdog.
//
// Example payload:
// {
//   "threshold": "3"
// }
type ConnectivityWatchdog struct {
	Metadata

	Threshold string `json:"threshold"`
}
