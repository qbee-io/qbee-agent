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

package software

import "fmt"

type Package struct {
	// Name - package name
	Name string `json:"name"`

	// Version - package version
	Version string `json:"version"`

	// Architecture - package architecture
	Architecture string `json:"arch,omitempty"`

	// Update - available package version upgrade
	Update string `json:"update,omitempty"`
}

// ID software package identifier.
func (pkg *Package) ID() string {
	if pkg.Architecture == "" {
		return pkg.Name
	}

	return fmt.Sprintf("%s:%s", pkg.Name, pkg.Architecture)
}
