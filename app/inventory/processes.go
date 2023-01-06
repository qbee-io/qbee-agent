package inventory

const TypeProcesses Type = "processes"

type Processes struct {
	Processes []Process `json:"items"`
}

type Process struct {
	// PID - process ID.
	PID int `json:"pid"`

	// User - owner of the process.
	User string `json:"user"`

	// Memory - memory usage in percent.
	Memory float64 `json:"mem"`

	// CPU - CPU usage in percent.
	CPU float64 `json:"cpu"`

	// Command - program command.
	Command string `json:"cmdline"`
}
