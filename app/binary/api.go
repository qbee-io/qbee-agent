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

package binary

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"

	"github.com/qbee-io/qbee-agent/app/api"
)

const downloadPath = "/v1/org/device/auth/download/%s/%s"

// download the latest binary version and return its metadata.
func download(ctx context.Context, apiClient *api.Client, name string, writer io.Writer) (*Metadata, error) {
	path := fmt.Sprintf(downloadPath, name, runtime.GOARCH)

	request, err := apiClient.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request: %v", err)
	}

	var response *http.Response
	if response, err = apiClient.Do(request); err != nil {
		return nil, fmt.Errorf("cannot fetch latest version: %v", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot fetch latest version: unexpected API response - %d", response.StatusCode)
	}

	metadata := &Metadata{
		Version:   response.Header.Get("X-Binary-Version"),
		Digest:    response.Header.Get("X-Binary-Digest"),
		Signature: response.Header.Get("X-Binary-Signature"),
	}

	if _, err = io.Copy(writer, response.Body); err != nil {
		return nil, fmt.Errorf("failed to download the agent binary: %v", err)
	}

	return metadata, nil
}
