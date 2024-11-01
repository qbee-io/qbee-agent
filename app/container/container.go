package container

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

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os/user"
	"path/filepath"
	"strings"

	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/utils"
)

type ContainerRuntimeType int

const (
	DockerRuntimeType ContainerRuntimeType = iota
	PodmanRuntimeType
)

// DockerContainerDirectory is where the agent will download docker related files.
const DockerContainerDirectory = "docker_containers"

// PodmanContainerDirectory is where the agent will download podman related files.
const PodmanContainerDirectory = "podman_containers"

// Container defines a docker container instance.
type Container struct {
	// ContainerRuntime defines the container runtime to be used.
	ContainerRuntime ContainerRuntimeType `json:"-"`

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

	cacheDirectory     string
	userCacheDirectory string

	user *user.User
}

// UserCheck checks if the user exists and sets it to the container.
func (c *Container) UserCheck() error {
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

// SetCacheDirectory sets the cache directory for the container.
func (c *Container) SetCacheDirectory(cacheDirectory, userCacheDirectory string) {
	c.cacheDirectory = cacheDirectory
	c.userCacheDirectory = userCacheDirectory
}

// GetCmdArgs returns docker cli command line arguments needed to launch the container.
func (c Container) GetCmdArgs() ([]string, error) {
	args := []string{
		"--name", c.Name,
	}

	envFilePath := c.LocalEnvFilePath()
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

// SetUserFilePermissions sets file permissions for the container env file.
func (c Container) SetUserFilePermissions(userCacheDirectory string) error {
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

// LocalEnvFilePath returns container specific local path of the envfile.
func (c Container) LocalEnvFilePath() string {
	if c.EnvFile == "" {
		return ""
	}

	cachePath := c.cacheDirectory
	if c.user != nil {
		cachePath = filepath.Join(c.userCacheDirectory, c.user.Uid)
	}

	if c.ContainerRuntime == PodmanRuntimeType {
		return filepath.Join(cachePath, PodmanContainerDirectory, fmt.Sprintf("%s.envfile", c.id()))
	}

	return filepath.Join(cachePath, DockerContainerDirectory, fmt.Sprintf("%s.envfile", c.id()))
}

// start a container
func (c Container) Run(ctx context.Context, containerBin string) ([]byte, error) {
	runCmd, err := c.getRunCommand(containerBin)
	if err != nil {
		return nil, err
	}

	output, err := c.runContainerCmd(ctx, runCmd)
	if err != nil {
		return nil, err
	}

	return output, nil
}

func (c Container) runContainerCmd(ctx context.Context, cmd []string) ([]byte, error) {
	if c.user == nil {
		return utils.RunCommand(ctx, cmd)
	}

	return utils.RunCommandAsUser(ctx, cmd, c.user)
}

// getRunCommand returns run command string for the container.
func (c Container) getRunCommand(containerBin string) ([]string, error) {
	args, err := c.GetCmdArgs()

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
func (c Container) Restart(ctx context.Context, containerBin, containerID string) ([]byte, error) {

	runCmd, err := c.getRunCommand(containerBin)
	if err != nil {
		return nil, err
	}

	c.kill(ctx, containerID, containerBin)

	output, err := c.runContainerCmd(ctx, runCmd)
	if err != nil {
		return nil, err
	}
	return output, nil
}

type ContainerInfo struct {
	ID     string            `json:"ID"`
	Labels map[string]string `json:"Labels"`
	State  string            `json:"State"`
}

// IsRunning returns true if container is currently running.
func (ci *ContainerInfo) IsRunning() bool {
	return ci.State == "running"
}

// exists returns true if container exists (regardless of its state).
func (ci *ContainerInfo) Exists() bool {
	return ci.ID != ""
}

// returns true if container is running with the right set of run arguments.
func (ci *ContainerInfo) ArgsMatch(args []string) bool {
	expectedArgsDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(args, " "))))

	if val, ok := ci.Labels["qbee-docker-args-sha"]; ok {
		return val == expectedArgsDigest
	}
	return false
}

// getStatus returns a status for
func (c Container) GetStatus(ctx context.Context, containerBin string) (*ContainerInfo, error) {

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

	ci := new(ContainerInfo)
	if bytes.TrimSpace(output) == nil {
		return ci, nil
	}

	if c.ContainerRuntime == PodmanRuntimeType {
		return c.parseStatusPodman(output)
	}

	if c.ContainerRuntime == DockerRuntimeType {
		return c.parseStatusDocker(output)
	}

	return nil, fmt.Errorf("unsupported container runtime: %s", containerBin)
}

// parseStatusPodman returns a status for a container using podman command.
func (c Container) parseStatusPodman(output []byte) (*ContainerInfo, error) {

	containers := make([]ContainerInfo, 0)

	if err := json.Unmarshal(output, &containers); err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return new(ContainerInfo), nil
	}

	return &containers[0], nil
}

// getStatusDocker returns a status for a container using docker command.
func (c Container) parseStatusDocker(output []byte) (*ContainerInfo, error) {

	var dockerContainer struct {
		ID     string `json:"ID"`
		Labels string `json:"Labels"`
		State  string `json:"State"`
	}

	if err := json.Unmarshal(output, &dockerContainer); err != nil {
		return nil, err
	}

	ci := new(ContainerInfo)

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
