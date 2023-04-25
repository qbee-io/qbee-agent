package configuration

import (
	"context"
	"testing"
	"time"

	"qbee.io/platform/shared/test/assert"

	"github.com/qbee-io/qbee-agent/app/api"
)

func TestService_reportsBuffer(t *testing.T) {
	srv := New(nil, t.TempDir(), "")

	t.Run("read empty buffer", func(t *testing.T) {
		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Empty(t, bufferedReports)
		}
	})

	t.Run("add expired reports", func(t *testing.T) {
		expiredReports := []Report{{
			Text:      "expired report",
			Timestamp: time.Now().Add(-reportsBufferExpiration - time.Second).Unix(),
		}}

		if err := srv.addReportsToBuffer(expiredReports); err != nil {
			t.Fatalf("failed to add reports to buffer: %v", err)
		}

		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Empty(t, bufferedReports)
		}
	})

	newReports := []Report{{Text: "new report 1", Timestamp: time.Now().Unix()}}

	t.Run("add fresh reports", func(t *testing.T) {
		if err := srv.addReportsToBuffer(newReports); err != nil {
			t.Fatalf("failed to add reports to buffer: %v", err)
		}

		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Equal(t, bufferedReports, newReports)
		}
	})

	t.Run("add more fresh reports", func(t *testing.T) {
		newReports2 := []Report{{Text: "new report 2", Timestamp: time.Now().Unix()}}
		if err := srv.addReportsToBuffer(newReports2); err != nil {
			t.Fatalf("failed to add reports to buffer: %v", err)
		}

		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Equal(t, bufferedReports, append(newReports, newReports2...))
		}
	})

	t.Run("clearing the buffer", func(t *testing.T) {
		// clearing the buffer wipes all added reports
		if err := srv.clearReportsBuffer(); err != nil {
			t.Fatalf("failed to clear reports buffer: %v", err)
		}

		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			assert.Empty(t, bufferedReports)
		}

		// clearing empty buffer shouldn't fail
		if err := srv.clearReportsBuffer(); err != nil {
			t.Fatalf("failed to clear reports buffer: %v", err)
		}
	})
}

func TestService_persistConfig(t *testing.T) {
	apiClient := api.NewClient("invalid-host.example", "12345", nil)
	srv := New(apiClient, t.TempDir(), "")

	cfg := &CommittedConfig{
		CommitID: "abc",
		Bundles:  []string{BundleSettings},
		BundleData: BundleData{
			Settings: SettingsBundle{
				RunInterval: 10,
			},
		},
	}

	srv.persistConfig(cfg)

	t.Run("load config from file", func(t *testing.T) {
		loadedCfg := new(CommittedConfig)

		if err := srv.loadConfig(loadedCfg); err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		assert.Equal(t, loadedCfg, cfg)
	})

	t.Run("load config through public Get method", func(t *testing.T) {
		committedConfig, err := srv.Get(context.Background())
		if err != nil {
			t.Fatalf("failed to get config: %v", err)
		}

		assert.Equal(t, committedConfig, cfg)
	})
}
