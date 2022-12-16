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
	PortsInventoryType  InventoryType = "ports"
)

// sendSystemInventory gathers system inventory and sends them to the device hub API.
func (agent *Agent) sendSystemInventory(ctx context.Context) error {
	systemInventory, err := inventory.CollectSystemInventory()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}

	systemInventory.System.AgentVersion = Version
	systemInventory.System.LastConfigCommitID = agent.lastConfigCommitID

	return agent.sendInventory(ctx, SystemInventoryType, systemInventory)
}

// sendPortsInventory gathers open ports inventory and sends them to the device hub API.
func (agent *Agent) sendPortsInventory(ctx context.Context) error {
	portsInventory, err := inventory.CollectPortsInventory()
	if err != nil {
		return fmt.Errorf("error collecting system info: %w", err)
	}

	return agent.sendInventory(ctx, PortsInventoryType, portsInventory)
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

	endpoint := fmt.Sprintf("/v1/org/device/auth/inventory/%s", inventoryType)

	response, err := agent.apiRequest(ctx, http.MethodPut, endpoint, buf)
	if err != nil {
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
