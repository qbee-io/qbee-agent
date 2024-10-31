// Copyright 2024 qbee.io
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
	"os/user"
	"path/filepath"
	"strings"

	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/utils"
)

const dockerRuntimeType = "docker"
const podmanRuntimeType = "podman"

// Container defines a docker container instance.
type Container struct {
	// ContainerRuntime defines the container runtime to be used.
	ContainerRuntime string `json:"-"`

	// Name used by the container.
	Name string `json:"name"`

	// Image used by the container.
	Image string `json:"image"`

	// Args defines command line arguments for "docker run".
	Args string `json:"docker_args"`

	// EnvFile defines an env file (from file manager) to be used inside container.
	EnvFile string `json:"env_file"`

	// Command to be executed in the container.
	Command string `json:"command"`

	// PreCondition is a shell command that needs to be true before starting the container.
	PreCondition string `json:"pre_condition,omitempty"`

	// SkipRestart defines whether the container should be restarted if it's stopped
	SkipRestart bool `json:"skip_restart,omitempty"`

	// ExecUser defines the user to execute the container as. Podman only.
	ExecUser string `json:"exec_user,omitempty"`

	user *user.User
}

// execute ensures that configured container is running
func (c Container) execute(ctx context.Context, srv *Service, containerBin string) error {
	var err error
	var needRestart bool

	if !CheckPreCondition(ctx, c.PreCondition) {
		// skip container if pre-condition is not met
		return nil
	}

	if err = c.userCheck(); err != nil {
		ReportError(ctx, err, "Cannot check user for container %s.", c.Name)
		return err
	}

	envFilePath := c.localEnvFilePath(srv)
	if envFilePath != "" {
		if needRestart, err = srv.downloadFile(ctx, "", c.EnvFile, envFilePath); err != nil {
			return err
		}

		if err = c.setUserFilePermissions(srv.userCacheDirectory); err != nil {
			ReportError(ctx, err, "Cannot set file permissions for env file %s.", c.EnvFile)
			return err
		}
	}

	var container *containerInfo
	if container, err = c.getStatus(ctx, containerBin); err != nil {
		ReportError(ctx, err, "Cannot check status for image %s.", c.Image)
		return err
	}

	// start a new container if it doesn't exist
	if !container.exists() {
		return c.run(ctx, srv, containerBin)
	}

	if c.SkipRestart {
		return nil
	}

	args, err := c.args(srv)
	if err != nil {
		return err
	}

	if !container.isRunning() {
		ReportWarning(ctx, nil, "Container exited for image %s.", c.Image)
		needRestart = true
	} else if !container.argsMatch(args) {
		ReportWarning(ctx, nil, "Container configuration update detected for image %s.", c.Image)
		needRestart = true
	}

	if !needRestart {
		return nil
	}

	return c.restart(ctx, srv, containerBin, container.ID)
}

// userCheck checks if the user exists and sets it to the container.
func (c *Container) userCheck() error {
	if c.ExecUser == "" {
		return nil
	}

	u, err := user.Lookup(c.ExecUser)
	if err != nil {
		return err
	}

	if u.Uid == "0" {
		return nil
	}

	c.user = u
	return nil
}

// args returns docker cli command line arguments needed to launch the container.
func (c Container) args(srv *Service) ([]string, error) {
	args := []string{
		"--name", c.Name,
	}

	envFilePath := c.localEnvFilePath(srv)
	if envFilePath != "" {
		args = append(args, "--env-file", envFilePath)
	}

	extraArgs, err := utils.ParseCommandLine(c.Args)
	if err != nil {
		return nil, err
	}

	if len(extraArgs) != 0 {
		args = append(args, extraArgs...)
	}

	args = append(args, c.Image)

	containerCmd, err := utils.ParseCommandLine(c.Command)
	if err != nil {
		return nil, err
	}

	if len(containerCmd) != 0 {
		return append(args, containerCmd...), nil
	}
	return args, nil
}

// id returns container identifier base on its name.
func (c Container) id() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(c.Name)))
}

// setUserFilePermissions sets file permissions for the container env file.
func (c Container) setUserFilePermissions(userCacheDirectory string) error {
	if c.user == nil {
		return nil
	}

	cacheDir := filepath.Join(userCacheDirectory, c.user.Uid)

	return filepath.Walk(cacheDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return utils.SetFileOwner(path, c.user)
	})
}

// localEnvFilePath returns container specific local path of the envfile.
func (c Container) localEnvFilePath(srv *Service) string {
	if c.EnvFile == "" {
		return ""
	}

	cachePath := srv.cacheDirectory
	if c.user != nil {
		cachePath = filepath.Join(srv.userCacheDirectory, c.user.Uid)
	}

	if c.ContainerRuntime == podmanRuntimeType {
		return filepath.Join(cachePath, PodmanContainerDirectory, fmt.Sprintf("%s.envfile", c.id()))
	}

	return filepath.Join(cachePath, DockerContainerDirectory, fmt.Sprintf("%s.envfile", c.id()))
}

// start a container
func (c Container) run(ctx context.Context, srv *Service, containerBin string) error {
	runCmd, err := c.getRunCommand(srv, containerBin)
	if err != nil {
		return err
	}

	output, err := c.runContainerCmd(ctx, runCmd)
	if err != nil {
		ReportError(ctx, err, "Unable to start container for image %s.", c.Image)
		return err
	}

	ReportInfo(ctx, output, "Successfully started container for image %s.", c.Image)

	return nil
}

func (c Container) runContainerCmd(ctx context.Context, cmd []string) ([]byte, error) {
	if c.user == nil {
		return utils.RunCommand(ctx, cmd)
	}

	return utils.RunCommandAsUser(ctx, cmd, c.user)
}

// getRunCommand returns run command string for the container.
func (c Container) getRunCommand(srv *Service, containerBin string) ([]string, error) {
	args, err := c.args(srv)

	if err != nil {
		return nil, err
	}

	runCmd := []string{
		containerBin, "run",
		"--detach",
		"--label", fmt.Sprintf("qbee-docker-id=%s", c.Name),
		"--label", fmt.Sprintf("qbee-docker-args-sha=%x", sha256.Sum256([]byte(strings.Join(args, " ")))),
	}

	return append(runCmd, args...), nil
}

// kill and remove a container. Do not track errors.
func (c Container) kill(ctx context.Context, containerID, containerBin string) {
	cmd := []string{
		containerBin, "kill", containerID,
	}
	// Attempt to forcefully kill the container
	if _, err := c.runContainerCmd(ctx, cmd); err != nil {
		log.Errorf("Failed to kill container %s: %w", containerID, err)
	}

	// TODO: check if container is still present after kill
	cmd = []string{
		containerBin, "rm", containerID,
	}

	// Attempt to remove the container
	if _, err := c.runContainerCmd(ctx, cmd); err != nil {
		log.Errorf("Failed to remove container %s: %w", containerID, err)
	}
}

// restart an existing container
func (c Container) restart(ctx context.Context, srv *Service, containerBin, containerID string) error {

	runCmd, err := c.getRunCommand(srv, containerBin)
	if err != nil {
		ReportError(ctx, err, "Unable to get run command for image %s.", c.Image)
		return err
	}

	c.kill(ctx, containerID, containerBin)

	output, err := c.runContainerCmd(ctx, runCmd)
	if err != nil {
		ReportError(ctx, err, "Unable to restart container for image %s.", c.Image)
		return err
	}

	ReportInfo(ctx, output, "Successfully restarted container for image %s.", c.Image)

	return nil
}

type containerInfo struct {
	ID     string            `json:"ID"`
	Labels map[string]string `json:"Labels"`
	State  string            `json:"State"`
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
func (ci *containerInfo) argsMatch(args []string) bool {
	expectedArgsDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(args, " "))))

	if val, ok := ci.Labels["qbee-docker-args-sha"]; ok {
		return val == expectedArgsDigest
	}
	return false
}

// getStatus returns a status for
func (c Container) getStatus(ctx context.Context, containerBin string) (*containerInfo, error) {

	cmd := []string{
		containerBin,
		"container", "ls",
		"--all",
		"--no-trunc",
		"--filter", fmt.Sprintf("label=qbee-docker-id=%s", c.Name),
		"--format", "{{json .}}",
	}

	var err error
	var output []byte

	output, err = c.runContainerCmd(ctx, cmd)
	if err != nil {
		return nil, err
	}

	ci := new(containerInfo)
	if bytes.TrimSpace(output) == nil {
		return ci, nil
	}

	if c.ContainerRuntime == podmanRuntimeType {
		return c.parseStatusPodman(output)
	}

	if c.ContainerRuntime == dockerRuntimeType {
		return c.parseStatusDocker(output)
	}

	return nil, fmt.Errorf("unsupported container runtime: %s", containerBin)
}

// parseStatusPodman returns a status for a container using podman command.
func (c Container) parseStatusPodman(output []byte) (*containerInfo, error) {

	containers := make([]containerInfo, 0)

	if err := json.Unmarshal(output, &containers); err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return new(containerInfo), nil
	}

	return &containers[0], nil
}

// getStatusDocker returns a status for a container using docker command.
func (c Container) parseStatusDocker(output []byte) (*containerInfo, error) {

	var dockerContainer struct {
		ID     string `json:"ID"`
		Labels string `json:"Labels"`
		State  string `json:"State"`
	}

	if err := json.Unmarshal(output, &dockerContainer); err != nil {
		return nil, err
	}

	ci := new(containerInfo)

	ci.Labels = make(map[string]string)
	ci.ID = dockerContainer.ID
	ci.State = dockerContainer.State

	for _, label := range strings.Split(dockerContainer.Labels, ",") {
		parts := strings.Split(label, "=")
		if len(parts) != 2 {
			continue
		}

		ci.Labels[parts[0]] = parts[1]
	}
	return ci, nil
}

// RegistryAuth defines credentials for docker registry authentication.
type RegistryAuth struct {
	// ContainerRuntime defines the container runtime to be used.
	ContainerRuntime string `json:"-"`

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

const dockerHubURL = "https://index.docker.io/v1/"
const dockerConfigFilename = "/root/.docker/config.json"
const podmanConfigFilename = "/run/containers/0/auth.json"

// DockerConfig is used to read-only data about docker repository auths.
type DockerConfig struct {
	Auths map[string]struct {
		Auth string `json:"auth"`
	} `json:"auths"`
}

func (a RegistryAuth) execute(ctx context.Context, dockerBin string) error {
	dockerConfig := new(DockerConfig)

	if err := a.userCheck(); err != nil {
		ReportError(ctx, err, "Cannot check user for registry %s.", a.URL())
		return err
	}

	configFilename := a.getConfigFilename()

	// read and parse existing config file
	dockerConfigData, err := os.ReadFile(configFilename)
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

	// otherwise we need to add those credentials with login command
	cmd := []string{dockerBin, "login", "--username", a.Username, "--password", a.Password, a.URL()}
	output, err := a.execLogin(ctx, cmd)
	if err != nil {
		ReportError(ctx, err, "Unable to authenticate with %s repository.", a.URL())
		return err
	}

	ReportInfo(ctx, output, "Configured credentials for %s.", a.URL())
	return nil
}

func (a RegistryAuth) execLogin(ctx context.Context, cmd []string) ([]byte, error) {
	if a.user != nil {
		return utils.RunCommandAsUser(ctx, cmd, a.user)
	}
	return utils.RunCommand(ctx, cmd)

}

// URL returns registry server, unless it's empty, then the default docker hub URL.
func (a RegistryAuth) URL() string {
	if a.Server == "" {
		return dockerHubURL
	}

	return a.Server
}

// userCheck checks if the user exists and sets it to the container.
func (a *RegistryAuth) userCheck() error {
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
func (a RegistryAuth) matches(encodedCredentials string) bool {
	encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", a.Username, a.Password)))

	return encoded == encodedCredentials
}

func (a RegistryAuth) getConfigFilename() string {

	if a.ContainerRuntime == podmanRuntimeType {
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
