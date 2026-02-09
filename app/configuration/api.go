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

package configuration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/log"
)

const deviceConfigurationAPIPath = "/v1/org/device/auth/config"

// get retrieves currently committed device configuration from the device hub API.
func (srv *Service) get(ctx context.Context) (*CommittedConfig, error) {
	cfg, err := srv.getWithRetry(ctx)

	// set a flag if the config endpoint is unreachable to avoid running certain operations
	// (eg. commands that require network) which would flood the logs with errors
	srv.configEndpointUnreachable = err != nil

	srv.reportAPIError(ctx, err)

	if err != nil {
		return nil, err
	}
	return cfg, nil
}

const (
	minReconnectDelay = 6
	maxReconnectDelay = 10
)

func (srv *Service) getWithRetry(ctx context.Context) (*CommittedConfig, error) {

	var err error
	cfg := new(CommittedConfig)

	if srv.firstRunRetryCounter == 0 {
		err = srv.api.Get(ctx, deviceConfigurationAPIPath, cfg)
		return cfg, err
	}

	// retry on first run as network might not be ready yet
	for srv.firstRunRetryCounter > 0 {
		srv.firstRunRetryCounter--
		err = srv.api.Get(ctx, deviceConfigurationAPIPath, cfg)
		if err != nil {
			attempts := defaultFirstRunRetryCounter - srv.firstRunRetryCounter
			reconnectIn := minReconnectDelay + rand.Int63n(maxReconnectDelay-minReconnectDelay)
			log.Infof("error getting configuration (%d): %v - reconnecting in %d seconds", attempts, err, reconnectIn)
			time.Sleep(time.Duration(reconnectIn) * time.Second)
			continue
		}
		// successful connection, set retry counter to 0
		srv.firstRunRetryCounter = 0
		return cfg, nil
	}
	return nil, err
}

const fileManagerMetadataAPIPath = "/v1/org/device/auth/filemetadata/%s"

type fileMetadataResponse struct {
	Status string       `json:"status"`
	Data   FileMetadata `json:"data"`
}

// getFileMetadata returns metadata for a file in the file manager.
func (srv *Service) getFileMetadataFromAPI(ctx context.Context, src string) (*FileMetadata, error) {
	path := fmt.Sprintf(fileManagerMetadataAPIPath, src)

	fileMetadataResp := new(fileMetadataResponse)

	if err := srv.api.Get(ctx, path, fileMetadataResp); err != nil {
		wrappedErr := fmt.Errorf("error getting file metadata: %w", err)
		if errors.As(err, new(api.ConnectionError)) {
			return nil, api.NewConnectionError(wrappedErr)
		}

		return nil, wrappedErr
	}

	return &fileMetadataResp.Data, nil
}

const fileManagerAPIPath = "/v1/org/device/auth/files/%s"
const fileManagerPublicAPIPath = "/v1/org/device/public/files"

// getFile returns file reader.
func (srv *Service) getFileFromAPI(ctx context.Context, src string, offset int64) (io.ReadCloser, error) {
	path := fmt.Sprintf(fileManagerAPIPath, src)

	request, err := srv.api.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	if offset > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	var response *http.Response
	if response, err = srv.api.Do(request); err != nil {
		return nil, fmt.Errorf("error getting file: %w", err)
	}

	return response.Body, nil
}

const reportsAPIPath = "/v1/org/device/auth/report"
const reportsDeliveryBatchSize = 100

// sendReports delivers reports from a configuration execution.
// Returns number of reports successfully delivered.
func (srv *Service) sendReports(ctx context.Context, reports []Report) (int, error) {
	log.Debugf("sending %d reports", len(reports))

	delivered := 0

	if len(reports) == 0 {
		return delivered, nil
	}

	// attempt to deliver reports to the device hub
	for len(reports) > 0 {
		buf := new(bytes.Buffer)
		jsonEncoder := json.NewEncoder(buf)
		count := 0

		for _, report := range reports {
			if err := jsonEncoder.Encode(report); err != nil {
				return delivered, fmt.Errorf("error encoding report into JSON: %w", err)
			}

			if count++; count >= reportsDeliveryBatchSize {
				break
			}
		}

		log.Debugf("sending batch of %d reports", count)
		if err := srv.api.Post(ctx, reportsAPIPath, buf, nil); err != nil {
			return delivered, fmt.Errorf("error delivering reports: %w", err)
		}

		delivered += count
		reports = reports[count:]
	}

	return delivered, nil
}
