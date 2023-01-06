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

// Get returns currently committed device configuration.
func (srv *Service) Get(ctx context.Context) (*CommittedConfig, error) {
	cfg := new(CommittedConfig)

	if err := srv.api.Get(ctx, deviceConfigurationAPIPath, cfg); err != nil {
		return nil, fmt.Errorf("error getting device config: %w", err)
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

// sendReports delivers reports from a configuration execution.
func (srv *Service) sendReports(ctx context.Context, reports []Report) error {
	if len(reports) == 0 {
		return nil
	}

	buf := new(bytes.Buffer)
	jsonEncoder := json.NewEncoder(buf)

	for _, report := range reports {
		if err := jsonEncoder.Encode(report); err != nil {
			return fmt.Errorf("error encoding report into JSON: %w", err)
		}
	}

	if err := srv.api.Post(ctx, reportsAPIPath, buf, nil); err != nil {
		return fmt.Errorf("error delivering reports: %w", err)
	}

	return nil
}
