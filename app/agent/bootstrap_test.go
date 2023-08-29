package agent

import (
	"testing"

	"qbee.io/platform/test/assert"
	"qbee.io/platform/test/device"
)

// Test_Bootstrap_Device_Name tests the device name bootstrap parameter.
func Test_Bootstrap_Device_Name(t *testing.T) {
	r := device.New(t)
	r.Bootstrap("--device-name", "test-device-name")

	attributes, err := r.API.GetDeviceAttributes(r.DeviceID)

	assert.NoError(t, err)

	assert.Equal(t, attributes.DeviceName, "test-device-name")
}
