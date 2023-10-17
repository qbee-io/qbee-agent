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

package remoteaccess

import (
	"context"
	"fmt"
)

const vpnCertPath = "/v1/org/device/auth/vpncert"

// getCredentials retrieves the remote access credentials from the device hub.
func (s *Service) getCredentials(ctx context.Context) (*Credentials, error) {
	credentials := new(Credentials)

	if err := s.api.Get(ctx, vpnCertPath, &credentials); err != nil {
		return nil, err
	}

	if credentials.Status != "OK" {
		return nil, fmt.Errorf("failed to get remote access credentials")
	}

	return credentials, nil
}
