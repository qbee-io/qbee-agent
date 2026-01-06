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

package inventory

import (
	"context"
	"fmt"

	"go.qbee.io/agent/app/log"
	"go.qbee.io/agent/app/software"
)

// TypeSoftware is the inventory type for software information.
const TypeSoftware Type = "software"

// Software contains software package information for a supported package manager.
type Software struct {
	// PackageManager - type of package manager generating the report
	PackageManager software.PackageManagerType `json:"pkg_manager"`

	// Items - list of installed software
	Items []software.Package `json:"items"`
}

// CollectSoftwareInventory returns populated Software inventory based on current system status.
func CollectSoftwareInventory(ctx context.Context, elevationCmd []string) (*Software, error) {
	pkgManager := software.DefaultPackageManager
	if pkgManager == nil {
		log.Debugf("no supported package manager found")
		return nil, nil
	}

	pkgList, err := pkgManager.ListPackages(ctx, elevationCmd)
	if err != nil {
		return nil, fmt.Errorf("error listing packages: %w", err)
	}

	softwareInventory := &Software{
		PackageManager: pkgManager.Type(),
		Items:          pkgList,
	}

	return softwareInventory, nil
}
