package inventory

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCollectDockerContainersInventory(t *testing.T) {
	dockerContainers, err := CollectDockerContainersInventory()
	if err != nil {
		t.Fatalf("error collecting docker containers: %v", err)
	}

	data, _ := json.MarshalIndent(dockerContainers, " ", " ")

	fmt.Println(string(data))
}
