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

package attributes

import (
	"context"
	"fmt"

	"go.qbee.io/agent/app/api"
)

const attributesAPIPath = "/v1/org/device/auth/attributes"

// Service provides methods for managing device attributes.
type Service struct {
	api *api.Client
}

// New returns a new attributes Service.
func New(apiClient *api.Client) *Service {
	return &Service{api: apiClient}
}

// Get retrieves current device attributes from the device hub.
func (srv *Service) Get(ctx context.Context) (*DeviceAttributes, error) {
	var response DeviceAttributes

	if err := srv.api.Get(ctx, attributesAPIPath, &response); err != nil {
		return nil, fmt.Errorf("error getting attributes: %w", err)
	}

	return &response, nil
}

// Set updates device attributes. Only the supplied attributes are sent to the API.
// An empty or null Value signals that the attribute should be deleted.
func (srv *Service) Set(ctx context.Context, attrs *DeviceAttributes) error {
	if err := srv.api.Patch(ctx, attributesAPIPath, attrs, nil); err != nil {
		return fmt.Errorf("error setting attributes: %w", err)
	}

	return nil
}
