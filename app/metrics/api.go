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

package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// Send delivers metrics to the device hub.
func (srv *Service) Send(ctx context.Context, metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	path := "/v1/org/device/auth/metric"

	buf := new(bytes.Buffer)

	encoder := json.NewEncoder(buf)

	for _, metric := range metrics {
		if err := encoder.Encode(metric); err != nil {
			return fmt.Errorf("error encoding metrics payload: %w", err)
		}
	}

	if err := srv.api.Post(ctx, path, buf, nil); err != nil {
		return fmt.Errorf("error sending metrics request: %w", err)
	}

	return nil
}
