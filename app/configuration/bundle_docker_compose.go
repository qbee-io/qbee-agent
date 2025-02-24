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

// DockerComposeMinimumVersion is the minimum version of docker compose that is supported.
const dockerComposeMinimumVersion = "2.0.0"

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

	version, err := d.ParseVersion(string(output))
	if err != nil {
		ReportError(ctx, err, "Cannot parse Docker Compose version")
		return err
	}

	if !utils.IsNewerVersionOrEqual(version, dockerComposeMinimumVersion) {
		ReportError(ctx, err, "Docker Compose version %s is not supported. Minimum version is %s", version, dockerComposeMinimumVersion)
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

	runningProjects, err := d.getLocalStatus(ctx)
	if err != nil {
		ReportError(ctx, err, "Cannot get list of running compose projects")
		return err
	}

	// clean projects first to release resources
	if err := d.clean(ctx, service, configuredProjects, runningProjects); err != nil {
		ReportError(ctx, err, "Cannot clean up compose projects")
		return err
	}

	for _, project := range d.Projects {
		if !CheckPreCondition(ctx, project.PreCondition) {
			continue
		}

		created, err := project.getResources(ctx, service)
		if err != nil {
			ReportError(ctx, err, "Cannot get resources for compose project %s", project.Name)
			return err
		}

		restart := false
		if runningProject, ok := runningProjects[project.Name]; ok {
			restart = project.needsRestart(runningProject) && !project.SkipRestart
		} else {
			// if project is not running, we should start it
			restart = !project.SkipRestart
		}

		if restart && !created {
			ReportWarning(ctx, nil, "One or more containers in exited state for project %s. Restart scheduled", project.Name)
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

// projectStatus is a project that is running in the system.
type projectStatus struct {
	Name   string `json:"Name"`
	Status string `json:"Status"`
}

// GetMinimumVersion returns the minimum version of docker compose that is supported.
func (d DockerComposeBundle) GetMinimumVersion() string {
	return dockerComposeMinimumVersion
}

// ParseVersion parses the version of docker compose from the output.
func (d DockerComposeBundle) ParseVersion(output string) (string, error) {

	matches := dockerComposeVersionRE.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return "", fmt.Errorf("cannot determine docker compose version")
	}

	return matches[1], nil
}

func (c Compose) getResources(ctx context.Context, service *Service) (bool, error) {
	downloadedComposeFile, err := c.getComposeFile(ctx, service)
	if err != nil {
		return false, err
	}

	downloadedContextFile, err := c.getContext(ctx, service)
	if err != nil {
		return false, err
	}

	return downloadedComposeFile || downloadedContextFile, nil
}

func (c Compose) getComposeFile(ctx context.Context, service *Service) (bool, error) {

	projectDirectory := c.getProjectDirectory(service)
	if err := os.MkdirAll(projectDirectory, 0700); err != nil {
		ReportError(ctx, err, "Cannot create directory for compose project %s", c.Name)
		return false, err
	}

	composeFilePath := filepath.Join(projectDirectory, composeFile)

	parameters := templateParametersMap(c.Parameters)

	if len(parameters) > 0 {
		return service.downloadTemplateFile(ctx, "", c.File, composeFilePath, parameters)
	}
	return service.downloadFile(ctx, "", c.File, composeFilePath)
}

func (c Compose) getProjectDirectory(service *Service) string {
	return filepath.Join(service.cacheDirectory, DockerComposeDirectory, c.Name)
}

func (c Compose) getContext(ctx context.Context, service *Service) (bool, error) {

	if !c.UseContext {
		return false, nil
	}

	if c.Context == "" {
		return false, nil
	}

	if !utils.IsSupportedTarExtension(c.Context) {
		return false, fmt.Errorf("unsupported context file extension %s", c.Context)
	}

	contextState := filepath.Join(c.getProjectDirectory(service), "context-metadata.json")

	contextTmpFilename := strings.Join([]string{composeContext, utils.GetTarExtension(c.Context)}, ".")
	contextDst := filepath.Join(c.getProjectDirectory(service), "_tmp", contextTmpFilename)

	downloaded, err := downloadStateFileCompare(ctx, service, contextState, c.Context, contextDst)

	if err != nil {
		return false, err
	}

	if !downloaded {
		return false, nil
	}

	contextUnpackDir := filepath.Join(c.getProjectDirectory(service), composeContext)

	if err := utils.UnpackTar(contextDst, contextUnpackDir); err != nil {
		return false, err
	}

	if err := os.Remove(contextDst); err != nil {
		return false, err
	}

	return true, nil
}

func (d DockerComposeBundle) getLocalStatus(ctx context.Context) (map[string]projectStatus, error) {
	projectListingCmd := []string{"docker", "compose", "ls", "--all", "--format", "json"}
	output, err := utils.RunCommand(ctx, projectListingCmd)

	if err != nil {
		return nil, fmt.Errorf("cannot get list of running compose projects: %w", err)
	}

	var runningProjects []projectStatus
	if err := json.Unmarshal(output, &runningProjects); err != nil {
		return nil, fmt.Errorf("cannot parse list of running compose projects: %w", err)
	}

	projects := make(map[string]projectStatus)
	for _, project := range runningProjects {
		projects[project.Name] = project
	}

	return projects, nil
}

func (d DockerComposeBundle) clean(
	ctx context.Context,
	service *Service,
	configuredProjects map[string]Compose,
	runningProjects map[string]projectStatus,
) error {
	if !d.Clean {
		return nil
	}

	for _, project := range runningProjects {
		if _, ok := configuredProjects[project.Name]; ok {
			continue
		}

		// Skip projects not deployed by qbee
		if !project.isDeployed(service) {
			continue
		}

		_, err := project.remove(ctx, service, project.Name)
		if err != nil {
			return fmt.Errorf("cannot stop compose project %s: %w", project.Name, err)
		}
	}

	return nil
}

func (c Compose) needsRestart(project projectStatus) bool {
	return strings.Contains(project.Status, "exited")
}

func (p projectStatus) remove(ctx context.Context, service *Service, projectName string) ([]byte, error) {
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

func (p projectStatus) isDeployed(service *Service) bool {
	if _, err := os.Stat(filepath.Join(service.cacheDirectory, DockerComposeDirectory, p.Name)); err != nil {
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
