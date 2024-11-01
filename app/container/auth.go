package container

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"go.qbee.io/agent/app/utils"
)

// RegistryAuth defines credentials for docker registry authentication.
type RegistryAuth struct {
	// ContainerRuntime defines the container runtime to be used.
	ContainerRuntime ContainerRuntimeType `json:"-"`

	// Server hostname of the registry.
	// When server is empty, we will use Docker Hub: https://registry-1.docker.io/v2/
	Server string `json:"server"`

	// Username for the registry.
	Username string `json:"username"`

	// Password for the Username.
	Password string `json:"password"`

	// ExecUser defines the user to execute the container as. Podman only.
	ExecUser string `json:"exec_user,omitempty"`

	user *user.User
}

// default login values	for docker hub
const dockerDockerHubURL = "https://index.docker.io/v1/"
const podmanDockerHubURL = "index.docker.io"
const dockerConfigFilename = "/root/.docker/config.json"
const podmanConfigFilename = "/run/containers/0/auth.json"

// DockerConfig is used to read-only data about docker repository auths.
type DockerConfig struct {
	Auths map[string]struct {
		Auth string `json:"auth"`
	} `json:"auths"`
}

func (a RegistryAuth) ExecLogin(ctx context.Context, cmd []string) ([]byte, error) {
	if a.user != nil {
		return utils.RunCommandAsUser(ctx, cmd, a.user)
	}
	return utils.RunCommand(ctx, cmd)

}

// URL returns registry server, unless it's empty, then the default docker hub URL.
func (a RegistryAuth) URL() string {
	if a.Server != "" {
		return a.Server
	}

	if a.ContainerRuntime == PodmanRuntimeType {
		return podmanDockerHubURL
	}

	return dockerDockerHubURL
}

// UserCheck checks if the user exists and sets it to the container.
func (a *RegistryAuth) UserCheck() error {
	if a.ExecUser == "" {
		return nil
	}

	u, err := user.Lookup(a.ExecUser)
	if err != nil {
		return err
	}

	if u.Uid == "0" {
		return nil
	}

	a.user = u
	return nil
}

// matches checks whether current RegistryAuth matches provided encoded credentials.
func (a RegistryAuth) Matches(encodedCredentials string) bool {
	encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", a.Username, a.Password)))

	return encoded == encodedCredentials
}

func (a RegistryAuth) GetConfigFilename() string {

	if a.ContainerRuntime == PodmanRuntimeType {
		if a.user != nil {
			return a.getPodmanUserConfigFile()
		}
		return podmanConfigFilename
	}

	if a.user != nil {
		return filepath.Join(a.user.HomeDir, ".docker", "config.json")
	}
	return dockerConfigFilename
}

func (a RegistryAuth) getPodmanUserConfigFile() string {

	configFile := filepath.Join("/run", "user", a.user.Uid, "containers", "auth.json")
	if _, err := os.Stat(configFile); err == nil {
		return configFile
	}

	return filepath.Join("/tmp", "podman-run-"+a.user.Uid, "containers", "auth.json")
}
