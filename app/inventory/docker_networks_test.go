package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestCollectDockerNetworksInventory(t *testing.T) {
	dockerNetworks, err := CollectDockerNetworksInventory(context.Background())
	if err != nil {
		t.Fatalf("error collecting docker networks: %v", err)
	}

	data, _ := json.MarshalIndent(dockerNetworks, " ", " ")

	fmt.Println(string(data))
}
