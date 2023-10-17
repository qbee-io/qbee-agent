// Copyright 2023 qbee.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"encoding/json"
	"testing"

	"qbee.io/platform/api/frontend/client"
	"qbee.io/platform/test/assert"
	"qbee.io/platform/test/runner"
)

// Test_Bootstrap_Device_Name tests the device name bootstrap parameter.
func Test_Bootstrap_Device_Name(t *testing.T) {
	r := runner.New(t)
	r.Bootstrap("--device-name", "test-device-name")

	attributes, err := r.API.GetDeviceAttributes(r.DeviceID)

	assert.NoError(t, err)

	assert.Equal(t, attributes.DeviceName, "test-device-name")
}

func Test_Bootstrap_Automatic(t *testing.T) {
	r := runner.New(t)
	r.API = client.NewAuthenticated()

	bootstrapKeys, err := r.API.ListBootstrapKeys()
	assert.NoError(t, err)

	r.MustExec("mkdir", "-p", "/etc/qbee")

	configBytes, err := json.Marshal(&Config{
		BootstrapKey:    bootstrapKeys.First().ID,
		DeviceHubServer: "devicehub",
		DeviceHubPort:   "8443",
	})

	assert.NoError(t, err)

	r.CreateFile("/etc/qbee/qbee-agent.json", configBytes)

	r.MustExec("qbee-agent", "start", "-1")

	configBytes = r.ReadFile("/etc/qbee/qbee-agent.json")

	config := new(Config)
	err = json.Unmarshal(configBytes, config)
	assert.NoError(t, err)

	// Check that bootstrap key is not saved
	assert.Equal(t, config.BootstrapKey, "")

	// Runner is not bootstrapped  with Bootstrap(), so we need to set the device ID manually
	deviceID := r.GetDeviceID()

	// Check that device is indeed bootstrapped
	device, err := r.API.GroupTreeGetNode(deviceID)
	assert.NoError(t, err)
	assert.Equal(t, device.NodeID, deviceID)
}

func Test_Bootstrap_Automatic_DeviceName(t *testing.T) {
	r := runner.New(t)
	r.API = client.NewAuthenticated()

	bootstrapKeys, err := r.API.ListBootstrapKeys()
	assert.NoError(t, err)

	r.MustExec("mkdir", "-p", "/etc/qbee")

	configBytes, err := json.Marshal(&Config{
		BootstrapKey:        bootstrapKeys.First().ID,
		DeviceHubServer:     "devicehub",
		DeviceHubPort:       "8443",
		DeviceName:          "test-device-name",
		DisableRemoteAccess: true,
	})

	assert.NoError(t, err)

	r.CreateFile("/etc/qbee/qbee-agent.json", configBytes)

	r.MustExec("qbee-agent", "start", "-1")

	configBytes = r.ReadFile("/etc/qbee/qbee-agent.json")

	config := new(Config)
	err = json.Unmarshal(configBytes, config)
	assert.NoError(t, err)

	// Check that fields are empty after bootstrap
	assert.Equal(t, config.BootstrapKey, "")
	assert.Equal(t, config.DeviceName, "")

	// Check that remote access is disabled
	assert.Equal(t, config.DisableRemoteAccess, true)

	// Runner is not bootstrapped with Bootstrap(), so we need to set the device ID manually
	deviceID := r.GetDeviceID()

	// Check that device is indeed bootstrapped
	device, err := r.API.GroupTreeGetNode(deviceID)
	assert.NoError(t, err)
	assert.Equal(t, device.NodeID, deviceID)

	// Check that device name is set
	attributes, err := r.API.GetDeviceAttributes(device.NodeID)
	assert.NoError(t, err)
	assert.Equal(t, attributes.DeviceName, "test-device-name")
}
