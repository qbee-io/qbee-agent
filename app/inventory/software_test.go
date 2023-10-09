package inventory_test

import (
	"fmt"
	"testing"

	"qbee.io/platform/test/runner"
)

func TestCollectSoftwareInventory_Deb(t *testing.T) {
	r := runner.New(t)

	data := r.MustExec("qbee-agent", "inventory", "-t", "software", "-d")

	fmt.Println(string(data))
}
