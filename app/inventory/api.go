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
	"bytes"
	"context"
	"fmt"
)

// send delivers inventory to the device hub.
func (srv *Service) send(ctx context.Context, inventoryType Type, buf *bytes.Buffer) error {
	path := fmt.Sprintf("/v1/org/device/auth/inventory/%s", inventoryType)

	if err := srv.api.Put(ctx, path, buf, nil); err != nil {
		return fmt.Errorf("error sending %s inventory request: %w", inventoryType, err)
	}

	return nil
}
