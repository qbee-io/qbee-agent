package configuration

// ProcWatch ensures running process are running (or not).
//
// Example payload:
// {
//   "processes": [
//    {
//      "name": "presentProcess",
//      "policy": "Present",
//      "command": "start.sh"
//    },
//    {
//      "name": "absentProcess",
//      "policy": "Absent",
//      "command": "stop.sh"
//    }
//  ]
// }
type ProcWatch struct {
	Metadata

	Processes []ProcessWatcher `json:"processes"`
}

// ProcessWatcher defines a watcher for a process.
type ProcessWatcher struct {
	// Name of the process to watch.
	Name string `json:"name"`

	// Policy for the process.
	Policy ProcessPolicy `json:"policy"`

	// Command to use to get the process in the expected state.
	// For:
	// - ProcessPresent it should be a start command,
	// - ProcessAbsent it should be a stop command.
	Command string `json:"command"`
}

type ProcessPolicy string

const (
	ProcessPresent ProcessPolicy = "Present"
	ProcessAbsent  ProcessPolicy = "Absent"
)
