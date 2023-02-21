package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestCollectDockerContainersInventory(t *testing.T) {
	dockerContainers, err := CollectDockerContainersInventory(context.Background())
	if err != nil {
		t.Fatalf("error collecting docker containers: %v", err)
	}

	data, _ := json.MarshalIndent(dockerContainers, " ", " ")

	fmt.Println(string(data))
}
