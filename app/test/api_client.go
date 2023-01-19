package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
)

const defaultAPIBaseURL = "https://www.app.qbee-dev.qbee.io/api/v2"

// APIClient encapsulates communication with the qbee API.
type APIClient struct {
	baseURL   string
	authToken string

	httpClient *http.Client
}

// NewAPIClient returns a new instance of an authenticated APIClient.
func NewAPIClient() *APIClient {
	apiBaseURL := strings.TrimSuffix(os.Getenv("QBEE_BASE_URL"), "/")
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBaseURL
	}

	apiClient := &APIClient{
		baseURL:    apiBaseURL,
		httpClient: http.DefaultClient,
	}

	email := os.Getenv("QBEE_EMAIL")
	password := os.Getenv("QBEE_PASSWORD")

	if email == "" || password == "" {
		panic("QBEE_EMAIL and QBEE_PASSWORD must be set to run this test")
	}

	apiClient.Login(email, password)

	return apiClient
}

// GetDeviceHubHost returns device-hub host matching the API.
func (api *APIClient) GetDeviceHubHost() string {
	return os.Getenv("QBEE_DEVICE_HUB_HOST")
}

// GetDeviceHubPort returns device-hub port matching the API.
func (api *APIClient) GetDeviceHubPort() string {
	return os.Getenv("QBEE_DEVICE_HUB_PORT")
}

// Login authenticates the APIClient with provided email and password.
func (api *APIClient) Login(email, password string) {
	const path = "/login"

	request := map[string]string{
		"email":    email,
		"password": password,
	}

	response := make(map[string]string)

	api.request(http.MethodPost, path, request, &response)

	api.authToken = response["token"]
}

// NewBootstrapKey returns a test bootstrap key with auto-accept and group set.
func (api *APIClient) NewBootstrapKey() string {
	// request new key
	const path = "/bootstrapkey"

	response := make(map[string]any)

	api.request(http.MethodPost, path, nil, &response)

	var bootstrapKey string
	for key := range response {
		bootstrapKey = key
	}

	// update its settings to auto-accept and root group
	request := map[string]any{
		"auto_accept": true,
	}

	keyPath := path + "/" + bootstrapKey
	api.request(http.MethodPut, keyPath, request, nil)

	return bootstrapKey
}

// DeleteBootstrapKey from the system.
func (api *APIClient) DeleteBootstrapKey(key string) {
	path := "/bootstrapkey/" + key

	api.request(http.MethodDelete, path, nil, nil)
}

// AssignDeviceToGroup puts unassigned device to a group.
func (api *APIClient) AssignDeviceToGroup(deviceID, parentID string) {
	const path = "/grouptree"

	request := map[string]any{
		"changes": []map[string]any{
			{
				"action": "move",
				"data": map[string]any{
					"parent_id":     parentID,
					"old_parent_id": "unassigned_group",
					"node_id":       deviceID,
					"position":      0,
					"type":          "device",
				},
			},
		},
	}

	api.request(http.MethodPut, path, request, nil)
}

// ChangeConfig adds a new change to the configuration.
func (api *APIClient) ChangeConfig(nodeID, bundleName string, bundle any) {
	const path = "/change"

	request := map[string]any{
		"node_id":  nodeID,
		"formtype": bundleName,
		"config":   bundle,
	}

	api.request(http.MethodPost, path, request, nil)
}

// CommitConfig commits current config changes.
func (api *APIClient) CommitConfig() {
	const path = "/commit"

	request := map[string]string{
		"action":  "commit",
		"message": "test",
	}

	api.request(http.MethodPost, path, request, nil)
}

// DeleteDevice from the system.
func (api *APIClient) DeleteDevice(deviceID string) {
	path := "/inventory/" + deviceID

	api.request(http.MethodDelete, path, nil, nil)
}

// DeletePendingDevice from the system.
func (api *APIClient) DeletePendingDevice(deviceID string) {
	path := "/removeapprovedhost/" + deviceID

	api.request(http.MethodDelete, path, nil, nil)
}

// UploadFile to the file-manager.
func (api *APIClient) UploadFile(name string, contents []byte) {
	const path = "/file"

	buf := new(bytes.Buffer)
	multipartWriter := multipart.NewWriter(buf)

	if part, err := multipartWriter.CreateFormFile("file", name); err != nil {
		panic(err)
	} else {
		if _, err = part.Write(contents); err != nil {
			panic(err)
		}
	}

	if part, err := multipartWriter.CreateFormField("path"); err != nil {
		panic(err)
	} else {
		if _, err = part.Write([]byte("/")); err != nil {
			panic(err)
		}
	}

	if err := multipartWriter.Close(); err != nil {
		panic(err)
	}

	url := api.baseURL + path

	request, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		panic(err)
	}

	request.Header.Add("Content-Type", multipartWriter.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+api.authToken)

	var response *http.Response
	if response, err = api.httpClient.Do(request); err != nil {
		panic(err)
	}

	response.Body.Close()
}

// DeleteFile from the file-manager.
func (api *APIClient) DeleteFile(name string) {
	path := "/file?path=" + name

	api.request(http.MethodDelete, path, nil, nil)
}

// request sends an http request with optional JSON payload (src) and optionally decodes JSON response to dst.
func (api *APIClient) request(method, path string, src, dst any) {
	if !strings.HasPrefix(path, "/") {
		panic(fmt.Errorf("path %s must start with /", path))
	}

	var body io.ReadWriter

	if src != nil {
		body = new(bytes.Buffer)

		if err := json.NewEncoder(body).Encode(src); err != nil {
			panic(err)
		}
	}

	url := api.baseURL + path

	request, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}

	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	if api.authToken != "" {
		request.Header.Set("Authorization", "Bearer "+api.authToken)
	}

	var response *http.Response
	if response, err = api.httpClient.Do(request); err != nil {
		panic(err)
	}

	responseBody, _ := io.ReadAll(response.Body)

	response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		if len(responseBody) > 0 {
			fmt.Println(string(responseBody))
		}

		panic(fmt.Errorf("got an http error: %d", response.StatusCode))
	}

	if dst != nil {
		if err = json.Unmarshal(responseBody, dst); err != nil {
			fmt.Println(string(responseBody))
			panic(err)
		}
	}
}
