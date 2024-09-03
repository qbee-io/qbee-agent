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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"go.qbee.io/agent/app/utils"
)

// Example payload:
//
//	{
//		"items": [
//		  {
//	     "name": "project-a",
//	     "compose_file": "/path/to/docker-compose.yml",
//		  }
//		],
//	}
type DockerComposeBundle struct {
	Metadata

	// DockerCompose projects to be running in the system.
	Projects []DockerCompose `json:"items"`

	// RegistryAuths contains credentials to private docker registries.
	RegistryAuths []RegistryAuth `json:"registry_auths"`

	// Clean removes all docker compose projects that are not defined in the bundle.
	Clean bool `json:"clean,omitempty"`
}

// DockerCompose controls docker compose projects running in the system.
type DockerCompose struct {
	// Name of the project.
	Name string `json:"name"`

	// Path to the docker-compose file.
	ComposeFile string `json:"compose_file"`

	// PreCondition is a shell command that needs to be true before starting the container.
	PreCondition string `json:"pre_condition,omitempty"`
}

const dockerComposeMinimumVersion = "2.0.0"

var dockerComposeVersionRE = regexp.MustCompile(`Docker Compose version v([0-9.]+)`)

// Execute docker compose configuration bundle on the system.
func (d DockerComposeBundle) Execute(ctx context.Context, service *Service) error {

	configuredProjects := make(map[string]bool)

	output, err := utils.RunCommand(ctx, []string{"docker", "compose", "version"})
	if err != nil {
		ReportError(ctx, err, "Docker Compose is not installed")
		return err
	}

	if !dockerComposeVersionRE.MatchString(string(output)) {
		ReportError(ctx, err, "Docker Compose version could not be determined")
		return err
	}

	version := dockerComposeVersionRE.FindStringSubmatch(string(output))[1]
	if !utils.IsNewerVersionOrEqual(version, dockerComposeMinimumVersion) {
		ReportError(ctx, err, "Docker Compose version is too old")
		return err
	}

	for _, project := range d.Projects {
		project.Name = resolveParameters(ctx, project.Name)
		project.ComposeFile = resolveParameters(ctx, project.ComposeFile)
		project.PreCondition = resolveParameters(ctx, project.PreCondition)

		configuredProjects[project.Name] = true

		if !CheckPreCondition(ctx, project.PreCondition) {
			continue
		}

		composeFilePath := filepath.Join(service.cacheDirectory, DockerComposeDirectory, fmt.Sprintf("%s.yml", project.Name))
		created, err := service.downloadFile(ctx, "", project.ComposeFile, composeFilePath)
		if err != nil {
			return err
		}

		if created {
			dockerComposeStart := []string{
				"docker",
				"compose",
				"-p",
				project.Name,
				"-f",
				composeFilePath,
				"up",
				"-d",
				"--remove-orphans",
				"--wait",
			}

			output, err := utils.RunCommand(ctx, dockerComposeStart)
			if err != nil {
				ReportError(ctx, err, "Cannot start compose project %s", project.Name)
				return err
			}

			ReportInfo(ctx, output, "Started compose project %s", project.Name)
		}

	}

	if !d.Clean {
		return nil
	}

	if err := dockerComposeClean(ctx, service, configuredProjects); err != nil {
		ReportError(ctx, err, "Cannot clean up compose projects")
		return err
	}

	return nil
}

type DockerComposeProject struct {
	Name string `json:"Name"`
}

func dockerComposeClean(ctx context.Context, service *Service, configuredProjects map[string]bool) error {
	projectListingCmd := []string{"docker", "compose", "ls", "--all", "--format", "json"}
	output, err := utils.RunCommand(ctx, projectListingCmd)

	if err != nil {
		return fmt.Errorf("cannot get list of running compose projects: %w", err)
	}

	var runningProjects []DockerComposeProject
	if err := json.Unmarshal(output, &runningProjects); err != nil {
		return fmt.Errorf("cannot parse list of running compose projects: %w", err)
	}

	for _, project := range runningProjects {
		if _, ok := configuredProjects[project.Name]; ok {
			continue
		}

		// Skip projects not deployed by qbee
		if !dockerComposeIsDeployed(project.Name, service) {
			continue
		}

		_, err := dockerComposeRemoveProject(ctx, project.Name)
		if err != nil {
			return fmt.Errorf("cannot stop compose project %s: %w", project.Name, err)
		}
	}

	return nil
}

func dockerComposeRemoveProject(ctx context.Context, projectName string) ([]byte, error) {
	dockerComposeStop := []string{
		"docker",
		"compose",
		"-p",
		projectName,
		"down",
		"--remove-orphans",
	}

	return utils.RunCommand(ctx, dockerComposeStop)
}

func dockerComposeIsDeployed(projectName string, service *Service) bool {
	if _, err := os.Stat(filepath.Join(service.cacheDirectory, DockerComposeDirectory, fmt.Sprintf("%s.yml", projectName))); err != nil {
		return false
	}
	return true
}
