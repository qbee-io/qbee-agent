package metrics

type Label string

const (
	CPU         Label = "cpu"
	Memory      Label = "memory"
	Filesystem  Label = "filesystem"
	LoadAverage Label = "loadavg_weighted"
	Network     Label = "network"
)

// Metric defines the base metric data structure.
type Metric struct {
	// Label identifies the type of the metric.
	Label Label `json:"label"`

	// Timestamp defines when the metric was recorded.
	Timestamp int64 `json:"ts"`

	// ID is an optional metric identifier.
	ID string `json:"id,omitempty"`

	// Values contain metric values.
	Values Values `json:"values"`
}

// Values combines values from all labels in one struct.
// Values without data, won't be stored in database nor marshaled into JSON.
type Values struct {
	*CPUValues         `json:",omitempty"`
	*MemoryValues      `json:",omitempty"`
	*FilesystemValues  `json:",omitempty"`
	*LoadAverageValues `json:",omitempty"`
	*NetworkValues     `json:",omitempty"`
}
