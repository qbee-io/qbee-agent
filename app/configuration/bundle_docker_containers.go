package configuration

// DockerContainers controls docker containers running in the system.
//
// Example payload:
// {
//	"items": [
//	  {
//      "name": "container-a",
//      "image": "debian:stable",
//      "docker_args": "-v /path/to/data-volume:/data --hostname my-hostname",
//      "env_file": "/my-directory/my-envfile",
//      "command": "echo 'hello world!'"
//	  }
//	],
//  "registry_auths": [
//    {
//       "server": "gcr.io",
//       "username": "user",
//       "password": "seCre7"
//    }
//  ]
// }
type DockerContainers struct {
	Metadata

	// Containers to be running in the system.
	Containers []DockerContainer `json:"items"`

	// RegistryAuths contains credentials to private docker registries.
	RegistryAuths []RegistryAuth `json:"registry_auths"`
}

// DockerContainer defines a docker container instance.
type DockerContainer struct {
	// Name used by the container.
	Name string `json:"name"`

	// Image used by the container.
	Image string `json:"image"`

	// DockerArgs defines command line arguments for "docker run".
	DockerArgs string `json:"docker_args"`

	// EnvFile defines an env file (from file manager) to be used inside container.
	EnvFile string `json:"env_file"`

	// Command to be executed in the container.
	Command string `json:"command"`
}

// RegistryAuth defines credentials for docker registry authentication.
type RegistryAuth struct {
	// Server hostname of the registry.
	Server string `json:"server"`

	// Username for the registry.
	Username string `json:"username"`

	// Password for the Username.
	Password string `json:"password"`
}
