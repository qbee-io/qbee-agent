package inventory

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCollectDockerImagesInventory(t *testing.T) {
	dockerImages, err := CollectDockerImagesInventory()
	if err != nil {
		t.Fatalf("error collecting docker images: %v", err)
	}

	data, _ := json.MarshalIndent(dockerImages, " ", " ")

	fmt.Println(string(data))
}
