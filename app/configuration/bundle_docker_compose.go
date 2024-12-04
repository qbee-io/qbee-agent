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
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

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

// DockerComposeBundle controls docker compose projects running in the system.
type DockerComposeBundle struct {
	Metadata

	// DockerCompose projects to be running in the system.
	Projects []Compose `json:"items"`

	// RegistryAuths contains credentials to private docker registries.
	RegistryAuths []RegistryAuth `json:"registry_auths"`

	// Clean removes all docker compose projects that are not defined in the bundle.
	Clean bool `json:"clean,omitempty"`
}

var dockerComposeVersionRE = regexp.MustCompile(`Docker Compose version v?([0-9.]+)`)

const DockerComposeMinimumVersion = "2.0.0"

// Execute docker compose configuration bundle on the system.
func (d DockerComposeBundle) Execute(ctx context.Context, service *Service) error {

	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		ReportError(ctx, nil, "Docker compose configuration configured, but no docker executable found on system")
		return fmt.Errorf("docker compose not supported: %v", err)
	}

	output, err := utils.RunCommand(ctx, []string{"docker", "compose", "version"})
	if err != nil {
		ReportError(ctx, err, "Docker Compose is not installed")
		return err
	}

	version, err := DockerComposeParseVersion(string(output))
	if err != nil {
		ReportError(ctx, err, "Cannot parse Docker Compose version")
		return err
	}

	if !utils.IsNewerVersionOrEqual(version, DockerComposeMinimumVersion) {
		ReportError(ctx, err, "Docker Compose version %s is not supported. Minimum version is %s", version, DockerComposeMinimumVersion)
		return err
	}

	// populate all registry credentials
	for _, auth := range d.RegistryAuths {
		auth.ContainerRuntime = dockerRuntimeType
		auth.Server = resolveParameters(ctx, auth.Server)
		auth.Username = resolveParameters(ctx, auth.Username)
		auth.Password = resolveParameters(ctx, auth.Password)

		if err = auth.execute(ctx, dockerBin); err != nil {
			ReportError(ctx, err, "Unable to authenticate with %s repository.", auth.URL())
			return err
		}
	}

	configuredProjects := make(map[string]Compose)
	for _, project := range d.Projects {
		project.Name = resolveParameters(ctx, project.Name)
		project.File = resolveParameters(ctx, project.File)
		project.Context = resolveParameters(ctx, project.Context)
		project.PreCondition = resolveParameters(ctx, project.PreCondition)

		configuredProjects[project.Name] = project
	}

	runningProjects, err := d.dockerComposeGetProjectStatus(ctx)
	if err != nil {
		ReportError(ctx, err, "Cannot get list of running compose projects")
		return err
	}

	// clean projects first to release resources
	if err := d.dockerComposeClean(ctx, service, configuredProjects, runningProjects); err != nil {
		ReportError(ctx, err, "Cannot clean up compose projects")
		return err
	}

	for _, project := range d.Projects {
		if !CheckPreCondition(ctx, project.PreCondition) {
			continue
		}

		created, err := dockerComposeGetResources(ctx, service, project)
		if err != nil {
			ReportError(ctx, err, "Cannot get resources for compose project %s", project.Name)
			return err
		}

		restart := false
		if runningProject, ok := runningProjects[project.Name]; ok {
			restart = d.needsRestart(runningProject) && !project.SkipRestart
		}

		if restart {
			ReportWarning(ctx, nil, "One or more containers in exited state for project %s. Restart scehduled", project.Name)
		}

		if created || restart {
			dockerComposeStart := []string{
				"docker",
				"compose",
				"--project-name",
				project.Name,
				"--project-directory",
				filepath.Join(service.cacheDirectory, DockerComposeDirectory, project.Name, composeContext),
				"--file",
				filepath.Join(service.cacheDirectory, DockerComposeDirectory, project.Name, composeFile),
				"up",
				"--build",
				"--remove-orphans",
				"--wait",
				"--timeout",
				dockerComposeTimeout,
				"--timestamps",
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

	return nil
}

// dockerComposeProject is a project that is running in the system.
type dockerComposeProject struct {
	Name   string `json:"Name"`
	Status string `json:"Status"`
}

func DockerComposeParseVersion(output string) (string, error) {

	matches := dockerComposeVersionRE.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return "", fmt.Errorf("cannot determine docker compose version")
	}

	return matches[1], nil
}

func dockerComposeGetResources(ctx context.Context, service *Service, project Compose) (bool, error) {
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

func dockerComposeGetComposeFile(ctx context.Context, service *Service, project Compose) (bool, error) {

	projectDirectory := dockerComposeGetProjectDirectory(service, project)
	if err := os.MkdirAll(projectDirectory, 0700); err != nil {
		ReportError(ctx, err, "Cannot create directory for compose project %s", project.Name)
		return false, err
	}

	composeFilePath := filepath.Join(projectDirectory, composeFile)

	parameters := templateParametersMap(project.Parameters)

	if len(parameters) > 0 {
		return service.downloadTemplateFile(ctx, "", project.File, composeFilePath, parameters)
	}
	return service.downloadFile(ctx, "", project.File, composeFilePath)
}

func dockerComposeGetProjectDirectory(service *Service, project Compose) string {
	return filepath.Join(service.cacheDirectory, DockerComposeDirectory, project.Name)
}

func dockerComposeGetContext(ctx context.Context, service *Service, project Compose) (bool, error) {

	if !project.UseContext {
		return false, nil
	}

	if project.Context == "" {
		return false, nil
	}

	if !utils.IsSupportedTarExtension(project.Context) {
		return false, fmt.Errorf("unsupported context file extension %s", project.Context)
	}

	contextState := filepath.Join(dockerComposeGetProjectDirectory(service, project), "context-metadata.json")

	contextTmpFilename := strings.Join([]string{composeContext, utils.GetTarExtension(project.Context)}, ".")
	contextDst := filepath.Join(dockerComposeGetProjectDirectory(service, project), "_tmp", contextTmpFilename)

	downloaded, err := downloadStateFileCompare(ctx, service, contextState, project.Context, contextDst)

	if err != nil {
		return false, err
	}

	if !downloaded {
		return false, nil
	}

	contextUnpackDir := filepath.Join(dockerComposeGetProjectDirectory(service, project), composeContext)

	if err := utils.UnpackTar(contextDst, contextUnpackDir); err != nil {
		return false, err
	}

	if err := os.Remove(contextDst); err != nil {
		return false, err
	}

	return true, nil
}

func (d DockerComposeBundle) dockerComposeGetProjectStatus(ctx context.Context) (map[string]dockerComposeProject, error) {
	projectListingCmd := []string{"docker", "compose", "ls", "--all", "--format", "json"}
	output, err := utils.RunCommand(ctx, projectListingCmd)

	if err != nil {
		return nil, fmt.Errorf("cannot get list of running compose projects: %w", err)
	}

	var runningProjects []dockerComposeProject
	if err := json.Unmarshal(output, &runningProjects); err != nil {
		return nil, fmt.Errorf("cannot parse list of running compose projects: %w", err)
	}

	projects := make(map[string]dockerComposeProject)
	for _, project := range runningProjects {
		projects[project.Name] = project
	}

	return projects, nil
}

func (d DockerComposeBundle) dockerComposeClean(
	ctx context.Context,
	service *Service,
	configuredProjects map[string]Compose,
	runningProjects map[string]dockerComposeProject,
) error {
	if !d.Clean {
		return nil
	}

	for _, project := range runningProjects {
		if _, ok := configuredProjects[project.Name]; ok {
			continue
		}

		// Skip projects not deployed by qbee
		if !dockerComposeIsDeployed(project.Name, service) {
			continue
		}

		_, err := dockerComposeRemoveProject(ctx, service, project.Name)
		if err != nil {
			return fmt.Errorf("cannot stop compose project %s: %w", project.Name, err)
		}
	}

	return nil
}

func (d DockerComposeBundle) needsRestart(project dockerComposeProject) bool {
	return strings.Contains(project.Status, "exited")
}

func dockerComposeRemoveProject(ctx context.Context, service *Service, projectName string) ([]byte, error) {
	dockerComposeStop := []string{
		"docker",
		"compose",
		"--project-name",
		projectName,
		"down",
		"--remove-orphans",
		"--volumes",
		"--timeout",
		"60",
		"--rmi",
		"all",
	}

	if output, err := utils.RunCommand(ctx, dockerComposeStop); err != nil {
		return output, err
	}

	dockerComposeProjectDir := filepath.Join(service.cacheDirectory, DockerComposeDirectory, projectName)
	if err := os.RemoveAll(dockerComposeProjectDir); err != nil {
		return nil, err
	}
	return nil, nil
}

func dockerComposeIsDeployed(projectName string, service *Service) bool {
	if _, err := os.Stat(filepath.Join(service.cacheDirectory, DockerComposeDirectory, projectName)); err != nil {
		return false
	}
	return true
}

func downloadStateFileCompare(
	ctx context.Context,
	service *Service,
	stateFilePath,
	src,
	dst string,
) (bool, error) {

	fileMetadata, err := service.getFileMetadata(ctx, src)
	if err != nil {
		return false, err
	}

	stateDir := path.Dir(stateFilePath)
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		if err := os.MkdirAll(stateDir, 0700); err != nil {
			return false, err
		}
	}

	doDownload := false
	if _, err := os.Stat(stateFilePath); os.IsNotExist(err) {
		doDownload = true
	} else {
		stateBytes, err := os.ReadFile(stateFilePath)
		if err != nil {
			return false, err
		}

		var stateData FileMetadata
		if err := json.Unmarshal(stateBytes, &stateData); err != nil {
			return false, err
		}

		if stateData.SHA256() != fileMetadata.SHA256() {
			doDownload = true
		}
	}

	if doDownload {
		if _, err := service.downloadMetadataCompare(ctx, "", src, dst, fileMetadata); err != nil {
			return false, err
		}

		stateBytes, err := json.Marshal(fileMetadata)
		if err != nil {
			return false, err
		}

		if err := os.WriteFile(stateFilePath, stateBytes, 0600); err != nil {
			return false, err
		}
	}

	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return false, nil
	}
	return doDownload, nil
}
