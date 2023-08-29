package agent

import (
	"encoding/json"
	"testing"

	"qbee.io/platform/api/frontend/client"
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

func Test_Bootstrap_Automatic(t *testing.T) {
	r := device.New(t)
	r.API = client.NewAuthenticated()

	bootstrapKeys, err := r.API.ListBootstrapKeys()
	assert.NoError(t, err)

	r.MustExec("mkdir", "-p", "/etc/qbee")

	configBytes, err := json.Marshal(&Config{
		BootstrapKey:    bootstrapKeys.First().ID,
		DeviceHubServer: "devicehub",
		DeviceHubPort:   "8443",
	})

	r.CreateFile("/etc/qbee/qbee-agent.json", configBytes)

	r.MustExec("qbee-agent", "start", "-1")
}
