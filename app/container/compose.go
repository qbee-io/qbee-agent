package container

import (
	"context"
	"os"
	"os/user"
	"path/filepath"

	"go.qbee.io/agent/app/utils"
)

// Compose controls docker compose projects running in the system.
type Compose struct {
	// ContainerRuntimeType is the type of container runtime to use.
	ContainerRuntime ContainerRuntimeType `json:"-"`

	// Name of the project.
	Name string `json:"name"`

	// File to the docker-compose file.
	File string `json:"file"`

	// ComposeContent is the content any build context.
	Context string `json:"context,omitempty"`

	// PreCondition is a shell command that needs to be true before starting the container.
	PreCondition string `json:"pre_condition,omitempty"`

	cacheDirectory     string
	userCacheDirectory string
	user               *user.User
}

const ComposeFile = "compose.yml"
const ComposeContext = "context"
const composeTimeout = "60"

func ComposeRemoveProject(ctx context.Context, cacheDir, projectName string, runTimeType ContainerRuntimeType) ([]byte, error) {

	var stopCommand []string

	switch runTimeType {
	case DockerRuntimeType:
		stopCommand = []string{
			"docker",
			"compose",
		}
	case PodmanRuntimeType:
		stopCommand = []string{
			"podman-compose",
			"-f",
			filepath.Join(cacheDir, projectName, ComposeFile),
		}
	}

	args := []string{
		"--project-name",
		projectName,
		"down",
		"--volumes",
		"--timeout",
		composeTimeout,
	}

	if runTimeType == DockerRuntimeType {
		args = append(args, "--rmi", "all", "--remove-orphans")
	}

	stopCommand = append(stopCommand, args...)

	if output, err := utils.RunCommand(ctx, stopCommand); err != nil {
		return output, err
	}

	composeProjectDir := filepath.Join(cacheDir, projectName)
	if err := os.RemoveAll(composeProjectDir); err != nil {
		return nil, err
	}
	return nil, nil
}

func (c Compose) ComposeIsDeployed(projectName string) bool {
	if _, err := os.Stat(filepath.Join(c.ComposeGetProjectDirectory(), projectName)); err != nil {
		return false
	}
	return true
}

func (c Compose) ComposeStart(ctx context.Context) ([]byte, error) {

	projectDirectory := c.ComposeGetProjectDirectory()
	composeFilePath := filepath.Join(projectDirectory, ComposeFile)

	var startCommand []string

	switch c.ContainerRuntime {
	case DockerRuntimeType:
		startCommand = []string{
			"docker",
			"compose",
			"--project-name",
			c.Name,
			"--project-directory",
			projectDirectory,
		}
	case PodmanRuntimeType:
		startCommand = []string{
			"podman-compose",
			"--project-name",
			c.Name,
		}
	}

	args := []string{
		"--file",
		composeFilePath,
		"up",
		"--build",
		"--remove-orphans",
		"--timeout",
		composeTimeout,
		"--force-recreate",
	}

	if c.ContainerRuntime == DockerRuntimeType {
		args = append(args, "--timestamps", "--wait")
	}

	if c.ContainerRuntime == PodmanRuntimeType {
		args = append(args, "--detach")
	}

	startCommand = append(startCommand, args...)

	output, err := utils.RunCommand(ctx, startCommand)
	if err != nil {
		return nil, err
	}

	return output, nil
}

func (c Compose) ComposeGetProjectDirectory() string {
	return filepath.Join(c.cacheDirectory, c.Name)
}

func (c *Compose) SetCacheDirectory(cacheDirectory, userCacheDirectory string) {
	c.cacheDirectory = cacheDirectory
	c.userCacheDirectory = userCacheDirectory
}
