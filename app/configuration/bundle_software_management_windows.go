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

//go:build windows

package configuration

import "context"

type SoftwareManagementBundle struct {
	Metadata

	Items []Software `json:"items"`
}

// ConfigFile definition.
type ConfigFile struct {
	// ConfigTemplate defines a source template file from file manager.
	ConfigTemplate string `json:"config_template"`

	// ConfigLocation defines an absolute path in the system where file will be created.
	ConfigLocation string `json:"config_location"`
}

// ConfigFileParameter defines parameter to be used in ConfigFile.
type ConfigFileParameter struct {
	// Key defines parameters name.
	Key string `json:"key"`

	// Value defines parameters value.
	Value string `json:"value"`
}

// Software defines software to be maintained in the system.
type Software struct {
	// Package defines a package name to install.
	Package string `json:"package"`

	// ServiceName defines an optional service name (if empty, Package is used).
	ServiceName string `json:"service_name"`

	// PreCondition defines an optional command which needs to return 0 in order for the Software to be installed.
	PreCondition string `json:"pre_condition,omitempty"`

	// ConfigFiles to be created for the software.
	ConfigFiles []ConfigFile `json:"config_files"`

	// Parameters for the ConfigFiles templating.
	Parameters []ConfigFileParameter `json:"parameters"`
}

func (s SoftwareManagementBundle) Execute(ctx context.Context, srv *Service) error {
	return nil
}
