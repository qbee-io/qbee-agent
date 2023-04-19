package configuration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const deviceConfigurationAPIPath = "/v1/org/device/auth/config"

// get retrieves currently committed device configuration from the device hub API.
func (srv *Service) get(ctx context.Context) (*CommittedConfig, error) {
	cfg := new(CommittedConfig)

	err := srv.api.Get(ctx, deviceConfigurationAPIPath, cfg)

	srv.reportAPIError(ctx, err)

	if err != nil {
		return nil, err
	}

	return cfg, nil
}

const fileManagerMetadataAPIPath = "/v1/org/device/auth/filemetadata/%s"

type fileMetadataResponse struct {
	Status string       `json:"status"`
	Data   FileMetadata `json:"data"`
}

// getFileMetadata returns metadata for a file in the file manager.
func (srv *Service) getFileMetadata(ctx context.Context, src string) (*FileMetadata, error) {
	path := fmt.Sprintf(fileManagerMetadataAPIPath, src)

	fileMetadataResp := new(fileMetadataResponse)

	if err := srv.api.Get(ctx, path, fileMetadataResp); err != nil {
		return nil, fmt.Errorf("error getting file metadata: %w", err)
	}

	return &fileMetadataResp.Data, nil
}

const fileManagerAPIPath = "/v1/org/device/auth/files/%s"

// getFile returns file reader.
func (srv *Service) getFile(ctx context.Context, src string) (io.ReadCloser, error) {
	path := fmt.Sprintf(fileManagerAPIPath, src)

	request, err := srv.api.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
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

		if err := srv.api.Post(ctx, reportsAPIPath, buf, nil); err != nil {
			return delivered, fmt.Errorf("error delivering reports: %w", err)
		}

		delivered += count
	}

	return delivered, nil
}