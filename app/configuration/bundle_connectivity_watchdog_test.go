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

package configuration_test

import (
	"context"
	"errors"
	"testing"

	"go.qbee.io/agent/app/api"
	"go.qbee.io/agent/app/configuration"
)

func Test_ConnectivityWatchdog(t *testing.T) {
	apiClient := api.NewClient("invalid-host.example", "12345")
	service := configuration.New(apiClient, "", "")

	committedConfig := configuration.CommittedConfig{
		Bundles: []string{"connectivity_watchdog"},
		BundleData: configuration.BundleData{
			ConnectivityWatchdog: &configuration.ConnectivityWatchdogBundle{
				Metadata:  configuration.Metadata{Enabled: true},
				Threshold: "2",
			},
		},
	}

	ctx := context.Background()

	if err := service.Execute(ctx, &committedConfig); err != nil {
		t.Fatalf("error executing config: %v", err)
	}

	// first attempt shouldn't result in a set reboot flag
	if _, err := service.Get(ctx); !errors.As(err, new(api.ConnectionError)) {
		t.Fatalf("expected connection error, got %t", err)
	}

	if service.ShouldReboot() {
		t.Fatalf("unexpected should reboot flag")
	}

	// second attempt should result in a set reboot flag
	if _, err := service.Get(ctx); !errors.As(err, new(api.ConnectionError)) {
		t.Fatalf("expected connection error, got %t", err)
	}

	if !service.ShouldReboot() {
		t.Fatalf("should reboot flag not set")
	}

	// reset reboot flag and make sure that executing config without connectivity watchdog doesn't trigger the reboot
	service.ResetRebootAfterRun()

	if err := service.Execute(ctx, new(configuration.CommittedConfig)); err != nil {
		t.Fatalf("error executing config: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := service.Get(ctx); !errors.As(err, new(api.ConnectionError)) {
			t.Fatalf("expected connection error, got %t", err)
		}
	}

	if service.ShouldReboot() {
		t.Fatalf("unexpected should reboot flag")
	}
}
