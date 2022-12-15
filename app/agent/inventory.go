package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/qbee-io/qbee-agent/app/inventory"
)

type InventoryType string

const (
	SystemInventoryType InventoryType = "system"
)

type SystemInventory struct {
	System inventory.SystemInfo `json:"system"`
}

// sendSystemInventory gathers system inventory and sends them to the device hub API.
func (agent *Agent) sendSystemInventory(ctx context.Context) error {
	systemInfo, err := inventory.CollectSystemInfo()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}

	systemInfo.AgentVersion = Version
	systemInfo.LastConfigCommitID = agent.lastConfigCommitID

	systemInventory := SystemInventory{
		System: *systemInfo,
	}

	return agent.sendInventory(ctx, SystemInventoryType, systemInventory)
}

// sendInventory delivers inventory to device hub if it has changes since last delivery.
func (agent *Agent) sendInventory(ctx context.Context, inventoryType InventoryType, inventoryData any) error {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(inventoryData); err != nil {
		return fmt.Errorf("error marshaling %s inventory data: %w", inventoryType, err)
	}

	currentDigest := fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))

	// if previously delivered inventory matches current one, don't report it
	if previousDigest, ok := agent.deliveredInventoryDigests[inventoryType]; ok && previousDigest == currentDigest {
		return nil
	}

	endpoint := fmt.Sprintf(
		"https://%s:%s/v1/org/device/auth/inventory/%s",
		agent.cfg.DeviceHubServer, agent.cfg.DeviceHubPort, inventoryType)

	request, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, buf)
	if err != nil {
		return fmt.Errorf("error preparing %s inventory request: %w", inventoryType, err)
	}

	var response *http.Response
	if response, err = agent.HTTPClient().Do(request); err != nil {
		return fmt.Errorf("error sending %s inventory request: %w", inventoryType, err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(response.Body)
		return fmt.Errorf("unexpected response status: %d %s", response.StatusCode, responseBody)
	}

	agent.deliveredInventoryDigests[inventoryType] = currentDigest

	return nil
}
