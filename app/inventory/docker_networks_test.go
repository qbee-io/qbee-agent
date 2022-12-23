package inventory

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCollectDockerNetworksInventory(t *testing.T) {
	dockerNetworks, err := CollectDockerNetworksInventory()
	if err != nil {
		t.Fatalf("error collecting docker networks: %v", err)
	}

	data, _ := json.MarshalIndent(dockerNetworks, " ", " ")

	fmt.Println(string(data))
}
