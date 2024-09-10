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

/*

passing context:

docker compose --project-name testing --file $(pwd)/compose.yml --project-directory $(pwd)/context up --wait --force-recreate

We need some way of unpacking context
- new context means re-create
- new compose file means re-create
- store context hash in a file, delete tarball after unpacking
- Remove context if changed to make sure we have clean
*/

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

	// ComposeFile to the docker-compose file.
	ComposeFile string `json:"file"`

	// ComposeContent is the content any build context.
	ComposeContext string `json:"context,omitempty"`

	// PreCondition is a shell command that needs to be true before starting the container.
	PreCondition string `json:"pre_condition,omitempty"`
}

const dockerComposeMinimumVersion = "2.0.0"
const dockerComposeFile = "compose.yml"
const dockerComposeContext = "context"

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
		project.ComposeContext = resolveParameters(ctx, project.ComposeContext)
		project.PreCondition = resolveParameters(ctx, project.PreCondition)

		configuredProjects[project.Name] = true

		if !CheckPreCondition(ctx, project.PreCondition) {
			continue
		}

		created, err := dockerComposeGetResources(ctx, service, project)
		if err != nil {
			ReportError(ctx, err, "Cannot get resources for compose project %s", project.Name)
			return err
		}

		if created {
			dockerComposeStart := []string{
				"docker",
				"compose",
				"--project-name",
				project.Name,
				"--project-directory",
				filepath.Join(service.cacheDirectory, DockerComposeDirectory, project.Name, dockerComposeContext),
				"--file",
				filepath.Join(service.cacheDirectory, DockerComposeDirectory, project.Name, dockerComposeFile),
				"up",
				"--remove-orphans",
				"--wait",
				"--force-recreate",
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

func dockerComposeGetResources(ctx context.Context, service *Service, project DockerCompose) (bool, error) {
	downloadedComposeFile, err := dockerComposeGetComposeFile(ctx, service, project)
	if err != nil {
		return false, err
	}

	downloadedContextFile, err := dockerComposeGetContext(ctx, service, project)
	if err != nil {
		return false, err
	}

	return downloadedComposeFile || downloadedContextFile, nil
}

func dockerComposeGetComposeFile(ctx context.Context, service *Service, project DockerCompose) (bool, error) {
	projectDirectory := dockerComposeGetProjectDirectory(service, project)
	if err := os.MkdirAll(projectDirectory, 0700); err != nil {
		ReportError(ctx, err, "Cannot create directory for compose project %s", project.Name)
		return false, err
	}

	composeFilePath := filepath.Join(projectDirectory, dockerComposeFile)

	return service.downloadFile(ctx, "", project.ComposeFile, composeFilePath)
}

func dockerComposeGetProjectDirectory(service *Service, project DockerCompose) string {
	return filepath.Join(service.cacheDirectory, DockerComposeDirectory, project.Name)
}

func dockerComposeGetContext(ctx context.Context, service *Service, project DockerCompose) (bool, error) {
	contextDirectory := filepath.Join(dockerComposeGetProjectDirectory(service, project), dockerComposeContext)

	if err := os.MkdirAll(contextDirectory, 0700); err != nil {
		return false, err
	}

	if project.ComposeContext == "" {
		return false, nil
	}

	fileMetaData, err := service.getFileMetadata(ctx, project.ComposeContext)
	if err != nil {
		return false, err
	}

	if fileMetaData == nil {
		return false, fmt.Errorf("context file %s does not exist", project.ComposeContext)
	}

	if fileMetaData.SHA256() == "" {
		return false, fmt.Errorf("context file %s has no hash", project.ComposeContext)
	}

	contextTarPath := filepath.Join(
		dockerComposeGetProjectDirectory(service, project),
		fmt.Sprintf("%s.%s", dockerComposeContext, filepath.Ext(project.ComposeContext)),
	)

	fmt.Println("Downloading context file", project.ComposeContext, "to", contextTarPath)

	if _, err := service.downloadFile(ctx, fileMetaData.SHA256(), project.ComposeContext, contextTarPath); err != nil {
		return false, err
	}

	if err := dockerComposeContextUnpack(contextTarPath, contextDirectory); err != nil {
		return false, err
	}

	return true, nil
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
		"--project-name",
		projectName,
		"down",
		"--remove-orphans",
	}

	return utils.RunCommand(ctx, dockerComposeStop)
}

func dockerComposeIsDeployed(projectName string, service *Service) bool {
	if _, err := os.Stat(filepath.Join(service.cacheDirectory, DockerComposeDirectory, projectName)); err != nil {
		return false
	}
	return true
}

func dockerComposeContextUnpack(tarPath, destination string) error {

	if err := utils.UnpackTar(tarPath, destination); err != nil {
		return err
	}

	return nil
}
