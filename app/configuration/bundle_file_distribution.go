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
	"context"
	"fmt"
)

// FileDistributionBundle controls files in the system.
//
// Example payload:
//
//	{
//	 "files": [
//	   {
//	 	 "label": "my file set",
//	     "templates": [
//	       {
//	         "source": "demo_file.json",
//	         "destination": "/tmp/demo_file.json",
//	         "is_template": true
//	       }
//	     ],
//	     "parameters": [
//	       {
//	         "key": "VAR1",
//	         "value": "VAL1"
//	       }
//	     ],
//	     "command": "echo \"it worked!\""
//	   }
//	 ]
//	}
type FileDistributionBundle struct {
	Metadata

	FileSets []FileSet `json:"files"`
}

// FileSet defines a file set to be maintained in the system.
type FileSet struct {
	// Label is an optional label for the file set.
	Label string `json:"label"`

	// Files defines files to be created in the filesystem.
	Files []File `json:"templates"`

	// Parameters define values to be used for template files.
	TemplateParameters []TemplateParameter `json:"parameters"`

	// AfterCommand defines a command to be executed after files are saved on the filesystem.
	AfterCommand string `json:"command"`

	// PreCondition defines an optional command which needs to return 0 in order for the FileSet to be executed.
	PreCondition string `json:"pre_condition" bson:"pre_condition"`
}

// ParametersMap returns TemplateParameters as map.
func (fs *FileSet) ParametersMap() map[string]string {
	parameters := make(map[string]string)

	for _, param := range fs.TemplateParameters {
		parameters[param.Key] = param.Value
	}

	return parameters
}

// File defines a single file parameters.
type File struct {
	// Source full file path from the file manager.
	Source string `json:"source"`

	// Destination defines absolute path of the file in the filesystem.
	Destination string `json:"destination"`

	// IsTemplate defines whether the file should be processed by the templating engine.
	IsTemplate bool `json:"is_template"`
}

// TemplateParameter defines a single parameter used to replace placeholder in a template.
type TemplateParameter struct {
	// Key of the parameter used in files.
	Key string `json:"key"`

	// Value of the parameter which will replace Key placeholders.
	Value string `json:"value"`
}

// Execute file distribution config on the system.
func (fd FileDistributionBundle) Execute(ctx context.Context, service *Service) error {
	for _, fileSet := range fd.FileSets {
		if !CheckPreCondition(ctx, fileSet.PreCondition) {
			continue
		}

		parameters := fileSet.ParametersMap()
		anythingChanged := false

		for _, file := range fileSet.Files {
			var err error
			var fileSource string
			var fileDestination string

			if fileSource, err = resolveSourcePath(file.Source); err != nil {
				return fmt.Errorf("cannot resolve file path: %w", err)
			}

			if fileDestination, err = resolveDestinationPath(fileSource, file.Destination); err != nil {
				return fmt.Errorf("cannot resolve file path: %w", err)
			}

			var created bool

			if file.IsTemplate {
				created, err = service.downloadTemplateFile(ctx, fileSet.Label, fileSource, fileDestination, parameters)
			} else {
				created, err = service.downloadFile(ctx, fileSet.Label, fileSource, fileDestination)
			}

			if err != nil {
				return err
			}

			if created {
				anythingChanged = true
			}
		}

		if anythingChanged && fileSet.AfterCommand != "" {
			output, err := RunCommand(ctx, fileSet.AfterCommand)
			if err != nil {
				ReportError(ctx, output, msgWithLabel(fileSet.Label, "After command failed: %v", err))
				return err
			}

			ReportInfo(ctx, output, msgWithLabel(fileSet.Label, "Successfully executed after command"))
		}
	}

	return nil
}
