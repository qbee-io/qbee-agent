package inventory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/qbee-io/qbee-agent/app/api"
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
