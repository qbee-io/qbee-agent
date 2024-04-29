// Copyright 2023 qbee.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.qbee.io/agent/app/utils"
)

// DockerContainersBundle controls docker containers running in the system.
//
// Example payload:
//
//	{
//		"items": [
//		  {
//	     "name": "container-a",
//	     "image": "debian:stable",
//	     "docker_args": "-v /path/to/data-volume:/data --hostname my-hostname",
//	     "env_file": "/my-directory/my-envfile",
//	     "command": "echo 'hello world!'"
//		  }
//		],
//	 "registry_auths": [
//	   {
//	      "server": "gcr.io",
//	      "username": "user",
//	      "password": "seCre7"
//	   }
//	 ]
//	}
type DockerContainersBundle struct {
	Metadata

	// Containers to be running in the system.
	Containers []DockerContainer `json:"items"`

	// RegistryAuths contains credentials to private docker registries.
	RegistryAuths []RegistryAuth `json:"registry_auths"`
}

// Execute docker containers configuration bundle on the system.
func (d DockerContainersBundle) Execute(ctx context.Context, service *Service) error {
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		ReportError(ctx, nil, "Docker container configuration configured, but no docker executable found on system")
		return fmt.Errorf("docker not supported: %v", err)
	}

	// populate all registry credentials
	for _, auth := range d.RegistryAuths {
		auth.Server = resolveParameters(ctx, auth.Server)
		auth.Username = resolveParameters(ctx, auth.Username)
		auth.Password = resolveParameters(ctx, auth.Password)

		if err = auth.execute(ctx, dockerBin); err != nil {
			ReportError(ctx, err, "Unable to authenticate with %s repository.", auth.URL())
			return err
		}
	}

	for containerIndex, container := range d.Containers {
		container.Name = resolveParameters(ctx, container.Name)
		container.Image = resolveParameters(ctx, container.Image)
		container.DockerArgs = resolveParameters(ctx, container.DockerArgs)
		container.EnvFile = resolveParameters(ctx, container.EnvFile)
		container.Command = resolveParameters(ctx, container.Command)
		container.PreCondition = resolveParameters(ctx, container.PreCondition)

		// for containers with empty name, use its index
		if container.Name == "" {
			container.Name = fmt.Sprintf("%d", containerIndex)
		}

		if err = container.execute(ctx, service, dockerBin); err != nil {
			return err
		}
	}

	return nil
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

	// PreCondition is a shell command that needs to be true before starting the container.
	PreCondition string `json:"pre_condition,omitempty"`
}

// execute ensures that configured container is running
func (c DockerContainer) execute(ctx context.Context, srv *Service, dockerBin string) error {
	var err error
	var needRestart bool

	if !CheckPreCondition(ctx, c.PreCondition) {
		// skip container if pre-condition is not met
		return nil
	}

	envFilePath := c.localEnvFilePath(srv)
	if envFilePath != "" {
		if needRestart, err = srv.downloadFile(ctx, "", c.EnvFile, envFilePath); err != nil {
			return err
		}
	}

	var container *containerInfo
	if container, err = c.getStatus(ctx, dockerBin); err != nil {
		ReportError(ctx, err, "Cannot check status for image %s.", c.Image)
		return err
	}

	// start a new container if it doesn't exist
	if !container.exists() {
		return c.run(ctx, srv, dockerBin)
	}

	if !container.isRunning() {
		ReportWarning(ctx, nil, "Container exited for image %s.", c.Image)
		needRestart = true
	} else if !container.argsMatch(c.args(srv)) {
		ReportWarning(ctx, nil, "Container configuration update detected for image %s.", c.Image)
		needRestart = true
	}

	if !needRestart {
		return nil
	}

	return c.restart(ctx, srv, dockerBin, container.ID)
}

// args returns docker cli command line arguments needed to launch the container.
func (c DockerContainer) args(srv *Service) string {
	args := []string{
		"--name", c.Name,
	}

	envFilePath := c.localEnvFilePath(srv)
	if envFilePath != "" {
		args = append(args, "--envfile", envFilePath)
	}

	args = append(args, c.DockerArgs, c.Image, c.Command)

	return strings.Join(args, " ")
}

// id returns container identifier base on its name.
func (c DockerContainer) id() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(c.Name)))
}

// localEnvFilePath returns container specific local path of the envfile.
func (c DockerContainer) localEnvFilePath(srv *Service) string {
	if c.EnvFile == "" {
		return ""
	}

	return filepath.Join(srv.cacheDirectory, DockerContainerDirectory, fmt.Sprintf("%s.envfile", c.id()))
}

// start a container
func (c DockerContainer) run(ctx context.Context, srv *Service, dockerBin string) error {
	runCmd := c.getRunCommand(srv, dockerBin)

	cmd := []string{"sh", "-c", runCmd}

	output, err := utils.RunCommand(ctx, cmd)
	if err != nil {
		ReportError(ctx, err, "Unable to start container for image %s.", c.Image)
		return err
	}

	ReportInfo(ctx, output, "Successfully started container for image %s.", c.Image)

	return nil
}

// getRunCommand returns run command string for the container.
func (c DockerContainer) getRunCommand(srv *Service, dockerBin string) string {
	args := c.args(srv)

	runCmd := []string{
		dockerBin, "run",
		"--detach",
		"--label", fmt.Sprintf("qbee-docker-id=%s", c.Name),
		"--label", fmt.Sprintf("qbee-docker-args-sha=%x", sha256.Sum256([]byte(args))),
		args,
	}

	return strings.Join(runCmd, " ")
}

// restart an existing container
func (c DockerContainer) restart(ctx context.Context, srv *Service, dockerBin, containerID string) error {
	restartCmd := []string{
		dockerBin, "kill", containerID, ";", // kill the container
		dockerBin, "rm", containerID, ";", // remove the container
		c.getRunCommand(srv, dockerBin), // start the container
	}

	cmd := []string{"sh", "-c", strings.Join(restartCmd, " ")}

	output, err := utils.RunCommand(ctx, cmd)
	if err != nil {
		ReportError(ctx, err, "Unable to restart container for image %s.", c.Image)
		return err
	}

	ReportInfo(ctx, output, "Successfully restarted container for image %s.", c.Image)

	return nil
}

type containerInfo struct {
	ID     string `json:"ID"`
	Labels string `json:"Labels"`
	State  string `json:"State"`
}

// isRunning returns true if container is currently running.
func (ci *containerInfo) isRunning() bool {
	return ci.State == "running"
}

// exists returns true if container exists (regardless of its state).
func (ci *containerInfo) exists() bool {
	return ci.ID != ""
}

// returns true if container is running with the right set of run arguments.
func (ci *containerInfo) argsMatch(args string) bool {
	expectedLabel := fmt.Sprintf("qbee-docker-args-sha=%x", sha256.Sum256([]byte(args)))

	for _, label := range strings.Split(ci.Labels, ",") {
		if label == expectedLabel {
			return true
		}
	}

	return false
}

// getStatus returns a status for
func (c DockerContainer) getStatus(ctx context.Context, dockerBin string) (*containerInfo, error) {
	cmd := []string{
		dockerBin,
		"container", "ls",
		"--all",
		"--no-trunc",
		"--filter", fmt.Sprintf("label=qbee-docker-id=%s", c.Name),
		"--format", "{{json .}}",
	}

	output, err := utils.RunCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}

	ci := new(containerInfo)

	if bytes.TrimSpace(output) == nil {
		return ci, nil
	}

	if err = json.Unmarshal(output, ci); err != nil {
		return nil, err
	}

	return ci, nil
}

// RegistryAuth defines credentials for docker registry authentication.
type RegistryAuth struct {
	// Server hostname of the registry.
	// When server is empty, we will use Docker Hub: https://registry-1.docker.io/v2/
	Server string `json:"server"`

	// Username for the registry.
	Username string `json:"username"`

	// Password for the Username.
	Password string `json:"password"`
}

const dockerHubURL = "https://index.docker.io/v1/"
const dockerConfigFilename = "/root/.docker/config.json"

// DockerConfig is used to read-only data about docker repository auths.
type DockerConfig struct {
	Auths map[string]struct {
		Auth string `json:"auth"`
	} `json:"auths"`
}

func (a RegistryAuth) execute(ctx context.Context, dockerBin string) error {
	dockerConfig := new(DockerConfig)

	// read and parse existing config file
	dockerConfigData, err := os.ReadFile(dockerConfigFilename)
	if err != nil {
		// if files doesn't exist, we can continue with empty DockerConfig, otherwise we need to error out
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	} else {
		// if config file exists, let's decode it to see if we have the rights credentials set already
		if err = json.Unmarshal(dockerConfigData, &dockerConfig); err != nil {
			return err
		}
	}

	// if we have matching credentials set for a registry, we can return
	if encodedCredentials, ok := dockerConfig.Auths[a.URL()]; ok && a.matches(encodedCredentials.Auth) {
		return nil
	}

	// otherwise we need to add those credentials with `docker login` command
	cmd := []string{dockerBin, "login", "--username", a.Username, "--password", a.Password, a.URL()}

	var output []byte
	if output, err = utils.RunCommand(ctx, cmd); err != nil {
		return err
	}

	ReportInfo(ctx, output, "Configured credentials for %s.", a.URL())

	return nil
}

// URL returns registry server, unless it's empty, then the default docker hub URL.
func (a RegistryAuth) URL() string {
	if a.Server == "" {
		return dockerHubURL
	}

	return a.Server
}

// matches checks whether current RegistryAuth matches provided encoded credentials.
func (a RegistryAuth) matches(encodedCredentials string) bool {
	encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", a.Username, a.Password)))

	return encoded == encodedCredentials
}
