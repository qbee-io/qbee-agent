package configuration

// Compose controls docker compose projects running in the system.
type Compose struct {
	// Name of the project.
	Name string `json:"name"`

	// File to the docker-compose file.
	File string `json:"file"`

	// ComposeContent is the content any build context (tarball) that is needed for the compose file.
	// NB: It is not recommend using build context in production environments as it will not create
	// immutable deployments. Use it only for development purposes.
	Context string `json:"context,omitempty"`

	// PreCondition is a shell command that needs to be true before starting the container.
	PreCondition string `json:"pre_condition,omitempty"`

	// SkipRestart skips the restart of the container.
	SkipRestart bool `json:"skip_restart,omitempty"`

	// NoForceRecreate skips the force recreation of containers
	// whose configuration has not changed upon changes in project resources
	NoForceRecreate bool `json:"no_force_restart,omitempty"`

	// Parameters are the parameters that can be used in the compose file.
	Parameters []TemplateParameter `json:"parameters,omitempty"`

	// UseContext defines if build context should be used.
	UseContext bool `json:"use_context,omitempty"`
}

const composeFile = "compose.yml"
const composeContext = "context"
const dockerComposeTimeout = "60"
