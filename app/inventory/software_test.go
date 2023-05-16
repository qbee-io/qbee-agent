package inventory_test

import (
	"fmt"
	"testing"

	"qbee.io/platform/test/device"
)

func TestCollectSoftwareInventory_Deb(t *testing.T) {
	r := device.New(t)

	data := r.MustExec("qbee-agent", "inventory", "-t", "software", "-d")

	fmt.Println(string(data))
}
