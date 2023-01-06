package inventory

import (
	"bytes"
	"context"
	"fmt"
)

// send delivers inventory to the device hub.
func (srv *Service) send(ctx context.Context, inventoryType Type, buf *bytes.Buffer) error {
	path := fmt.Sprintf("/v1/org/device/auth/inventory/%s", inventoryType)

	if err := srv.api.Put(ctx, path, buf, nil); err != nil {
		return fmt.Errorf("error sending %s inventory request: %w", inventoryType, err)
	}

	return nil
}
