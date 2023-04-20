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
func download(apiClient *api.Client, ctx context.Context, name string, writer io.Writer) (*Metadata, error) {
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
