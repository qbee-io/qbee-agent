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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/log"
)

// Service provides methods for collecting and delivering inventory data.
type Service struct {
	api                           *api.Client
	deliveredInventoryDigests     map[Type]string
	deliveredInventoryDigestsLock sync.Mutex
}

// New returns a new instance of inventory Service.
func New(apiClient *api.Client) *Service {
	return &Service{
		api:                       apiClient,
		deliveredInventoryDigests: make(map[Type]string),
	}
}

// Send delivers inventory to device hub if it has changes since last delivery.
func (srv *Service) Send(ctx context.Context, inventoryType Type, inventoryData any) error {
	if inventoryData == nil {
		log.Debugf("no %s inventory data to send", inventoryType)
		return nil
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(inventoryData); err != nil {
		return fmt.Errorf("error marshaling %s inventory data: %w", inventoryType, err)
	}

	currentDigest := fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))

	// if previously delivered inventory matches current one, don't report it
	if previousDigest, ok := srv.deliveredInventoryDigests[inventoryType]; ok && previousDigest == currentDigest {
		return nil
	}

	if err := srv.send(ctx, inventoryType, buf); err != nil {
		return fmt.Errorf("error sending %s inventory request: %w", inventoryType, err)
	}

	srv.deliveredInventoryDigestsLock.Lock()
	srv.deliveredInventoryDigests[inventoryType] = currentDigest
	srv.deliveredInventoryDigestsLock.Unlock()

	return nil
}
