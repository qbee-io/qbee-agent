package inventory

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCollectDockerVolumesInventory(t *testing.T) {
	dockerVolumes, err := CollectDockerVolumesInventory(nil)
	if err != nil {
		t.Fatalf("error collecting docker volumes: %v", err)
	}

	data, _ := json.MarshalIndent(dockerVolumes, " ", " ")

	fmt.Println(string(data))
}
