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

package inventory

import (
	"context"

	"go.qbee.io/agent/app/image"
)

// TypeRauc is the inventory type of the RAUC inventory.
const TypeRauc Type = "rauc"

// CollectRaucInventory collects the RAUC inventory.
func CollectRaucInventory(ctx context.Context) (*image.RaucStatus, error) {
	if !image.HasRauc() {
		return nil, nil
	}

	raucVersion, err := image.GetRaucVersion(ctx)
	if err != nil {
		return nil, nil
	}

	isCompatible := image.IsRaucCompatible(raucVersion)
	if !isCompatible {
		return nil, nil
	}

	return image.GetRaucStatus(ctx)
}
