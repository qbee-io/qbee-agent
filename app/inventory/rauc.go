package inventory

import (
	"context"

	"go.qbee.io/agent/app/image"
)

const TypeRauc Type = "rauc"

func CollectRaucInventory(ctx context.Context) (*image.RaucInfo, error) {
	if !image.HasRauc() {
		return nil, nil
	}

	return image.GetRaucInfo(ctx)
}
