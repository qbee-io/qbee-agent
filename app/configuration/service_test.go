package configuration

import (
	"testing"
	"time"

	"github.com/qbee-io/qbee-agent/app/test"
)

func TestService_reportsBuffer(t *testing.T) {
	srv := New(nil, t.TempDir(), "")

	t.Run("read empty buffer", func(t *testing.T) {
		if bufferedReports, err := srv.readReportsBuffer(); err != nil {
			t.Fatalf("failed to read reports from buffer: %v", err)
		} else {
			test.Empty(t, bufferedReports)
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
			test.Empty(t, bufferedReports)
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
			test.Equal(t, bufferedReports, newReports)
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
			test.Equal(t, bufferedReports, append(newReports, newReports2...))
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
			test.Empty(t, bufferedReports)
		}

		// clearing empty buffer shouldn't fail
		if err := srv.clearReportsBuffer(); err != nil {
			t.Fatalf("failed to clear reports buffer: %v", err)
		}
	})
}
