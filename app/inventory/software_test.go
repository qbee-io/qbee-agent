package inventory_test

import (
	"fmt"
	"testing"

	"github.com/qbee-io/qbee-agent/app/test"
)

func TestCollectSoftwareInventory_Deb(t *testing.T) {
	r := test.New(t)

	data := r.MustExec("qbee-agent", "inventory", "-t", "software", "-d")

	fmt.Println(string(data))
}
