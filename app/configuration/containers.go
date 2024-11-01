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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.qbee.io/agent/app/container"
	"go.qbee.io/agent/app/utils"
)

// executeContainer ensures that configured container is running
func executeContainer(ctx context.Context, srv *Service, containerBin string, c container.Container) error {
	var err error
	var needRestart bool

	if !CheckPreCondition(ctx, c.PreCondition) {
		// skip container if pre-condition is not met
		return nil
	}

	if err = c.UserCheck(); err != nil {
		ReportError(ctx, err, "Cannot check user for container %s.", c.Name)
		return err
	}

	envFilePath := c.LocalEnvFilePath()
	if envFilePath != "" {
		if needRestart, err = srv.downloadFile(ctx, "", c.EnvFile, envFilePath); err != nil {
			return err
		}

		if err = c.SetUserFilePermissions(srv.userCacheDirectory); err != nil {
			ReportError(ctx, err, "Cannot set file permissions for env file %s.", c.EnvFile)
			return err
		}
	}

	var container *container.ContainerInfo
	if container, err = c.GetStatus(ctx, containerBin); err != nil {
		ReportError(ctx, err, "Cannot check status for image %s.", c.Image)
		return err
	}

	// start a new container if it doesn't exist
	if !container.Exists() {
		output, err := c.Run(ctx, containerBin)
		if err != nil {
			ReportError(ctx, err, "Cannot start container for image %s.", c.Image)
			return err
		}

		ReportInfo(ctx, output, "Successfully started container for image %s.", c.Image)
		return nil
	}

	if c.SkipRestart {
		return nil
	}

	args, err := c.GetCmdArgs()
	if err != nil {
		return err
	}

	if !container.IsRunning() {
		ReportWarning(ctx, nil, "Container exited for image %s.", c.Image)
		needRestart = true
	} else if !container.ArgsMatch(args) {
		ReportWarning(ctx, nil, "Container configuration update detected for image %s.", c.Image)
		needRestart = true
	}

	if !needRestart {
		return nil
	}

	output, err := c.Restart(ctx, containerBin, container.ID)
	if err != nil {
		ReportError(ctx, err, "Cannot restart container for image %s.", c.Image)
		return err
	}

	ReportInfo(ctx, output, "Successfully restarted container for image %s.", c.Image)
	return nil
}

// DockerConfig is used to read-only data about docker repository auths.
type DockerConfig struct {
	Auths map[string]struct {
		Auth string `json:"auth"`
	} `json:"auths"`
}

func executeAuth(ctx context.Context, dockerBin string, a container.RegistryAuth) error {
	dockerConfig := new(DockerConfig)

	if err := a.UserCheck(); err != nil {
		ReportError(ctx, err, "Cannot check user for registry %s.", a.URL())
		return err
	}

	configFilename := a.GetConfigFilename()

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
	if encodedCredentials, ok := dockerConfig.Auths[a.URL()]; ok && a.Matches(encodedCredentials.Auth) {
		return nil
	}

	// otherwise we need to add those credentials with login command
	cmd := []string{dockerBin, "login", "--username", a.Username, "--password", a.Password, a.URL()}
	output, err := a.ExecLogin(ctx, cmd)
	if err != nil {
		ReportError(ctx, err, "Unable to authenticate with %s repository.", a.URL())
		return err
	}

	ReportInfo(ctx, output, "Configured credentials for %s.", a.URL())
	return nil
}

func executeCompose(ctx context.Context, srv *Service, project container.Compose) error {

	if !CheckPreCondition(ctx, project.PreCondition) {
		return nil
	}

	created, err := composeGetResources(ctx, srv, project)
	if err != nil {
		ReportError(ctx, err, "Cannot get resources for compose project %s", project.Name)
		return err
	}

	if !created {
		return nil
	}

	output, err := project.ComposeStart(ctx)
	if err != nil {
		ReportError(ctx, err, "Cannot start compose project %s", project.Name)
		return err
	}

	ReportInfo(ctx, output, "Started compose project %s", project.Name)
	return nil
}

func composeGetResources(ctx context.Context, srv *Service, compose container.Compose) (bool, error) {
	downloadedComposeFile, err := composeGetComposeFile(ctx, srv, compose)
	if err != nil {
		return false, err
	}

	downloadedContextFile, err := composeGetContext(ctx, srv, compose)
	if err != nil {
		return false, err
	}

	return downloadedComposeFile || downloadedContextFile, nil
}

func composeGetComposeFile(ctx context.Context, srv *Service, project container.Compose) (bool, error) {
	projectDirectory := project.ComposeGetProjectDirectory()
	if err := os.MkdirAll(projectDirectory, 0700); err != nil {
		ReportError(ctx, err, "Cannot create directory for compose project %s", project.Name)
		return false, err
	}

	composeFilePath := filepath.Join(projectDirectory, container.ComposeFile)

	return srv.downloadFile(ctx, "", project.File, composeFilePath)
}

func composeGetContext(ctx context.Context, service *Service, project container.Compose) (bool, error) {

	if project.Context == "" {
		return false, nil
	}

	if !utils.IsSupportedTarExtension(project.Context) {
		return false, fmt.Errorf("unsupported context file extension %s", project.Context)
	}

	contextState := filepath.Join(project.ComposeGetProjectDirectory(), "context-metadata.json")

	contextTmpFilename := strings.Join([]string{container.ComposeContext, utils.GetTarExtension(project.Context)}, ".")
	contextDst := filepath.Join(project.ComposeGetProjectDirectory(), "_tmp", contextTmpFilename)

	downloaded, err := downloadStateFileCompare(ctx, service, contextState, project.Context, contextDst)

	if err != nil {
		return false, err
	}

	if !downloaded {
		return false, nil
	}

	contextUnpackDir := filepath.Join(project.ComposeGetProjectDirectory(), container.ComposeContext)

	if err := utils.UnpackTar(contextDst, contextUnpackDir); err != nil {
		return false, err
	}

	if err := os.Remove(contextDst); err != nil {
		return false, err
	}

	return true, nil
}

func composeClean(ctx context.Context, cachepath, userCachePath string, configuredProjects map[string]container.Compose, runTimeType container.ContainerRuntimeType) error {

	if _, err := os.Stat(cachepath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cachePathProjects, err := os.ReadDir(cachepath)
	if err != nil {
		return fmt.Errorf("cannot list cache directory: %w", err)
	}

	for _, path := range cachePathProjects {
		if !path.IsDir() {
			continue
		}

		if _, ok := configuredProjects[path.Name()]; ok {
			continue
		}

		output, err := container.ComposeRemoveProject(ctx, userCachePath, path.Name(), runTimeType)
		if err != nil {
			ReportError(ctx, err, "Cannot remove compose project %s", path.Name())
			return err
		}

		ReportInfo(ctx, output, "Removed unconfigured compose project %s", path.Name())
	}
	return nil
}
