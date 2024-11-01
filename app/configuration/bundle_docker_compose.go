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
	"path/filepath"
	"regexp"

	"go.qbee.io/agent/app/container"
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

// DockerComposeBundle controls docker compose projects running in the system.
type DockerComposeBundle struct {
	Metadata

	// DockerCompose projects to be running in the system.
	Projects []container.Compose `json:"items"`

	// RegistryAuths contains credentials to private docker registries.
	RegistryAuths []container.RegistryAuth `json:"registry_auths"`

	// Clean removes all docker compose projects that are not defined in the bundle.
	Clean bool `json:"clean,omitempty"`
}

var dockerComposeVersionRE = regexp.MustCompile(`Docker Compose version v([0-9.]+)`)

const dockerComposeMinimumVersion = "2.0.0"

// Execute docker compose configuration bundle on the system.
func (c DockerComposeBundle) Execute(ctx context.Context, service *Service) error {

	configuredProjects := make(map[string]container.Compose)

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
		ReportError(ctx, err, "Docker Compose version %s is not supported. Minimum version is %s", version, dockerComposeMinimumVersion)
		return err
	}

	for _, project := range c.Projects {
		project.ContainerRuntime = container.DockerRuntimeType
		project.Name = resolveParameters(ctx, project.Name)
		project.File = resolveParameters(ctx, project.File)
		project.Context = resolveParameters(ctx, project.Context)
		project.PreCondition = resolveParameters(ctx, project.PreCondition)

		configuredProjects[project.Name] = project
	}

	cacheDirectory := filepath.Join(service.cacheDirectory, DockerComposeDirectory)
	userCacheDirectory := filepath.Join(service.userCacheDirectory, DockerComposeDirectory)

	if c.Clean {
		if err := composeClean(ctx, cacheDirectory, userCacheDirectory, configuredProjects, container.DockerRuntimeType); err != nil {
			ReportError(ctx, err, "Cannot clean up compose projects")
			return err
		}
	}

	for _, project := range c.Projects {

		project.SetCacheDirectory(cacheDirectory, userCacheDirectory)

		if err := executeCompose(ctx, service, project); err != nil {
			return err
		}
	}

	return nil
}
