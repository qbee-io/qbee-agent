package configuration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/qbee-io/qbee-agent/app/api"
	"github.com/qbee-io/qbee-agent/app/configuration"
)

func Test_ConnectivityWatchdog(t *testing.T) {
	apiClient := api.NewClient("invalid-host.example", "12345", nil)
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
