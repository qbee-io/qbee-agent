package configuration

import (
	"testing"
	"time"

	"github.com/qbee-io/qbee-agent/app/test"
)

func TestService_addReportsToBuffer(t *testing.T) {
	srv := New(nil, "")

	// adding expired reports should work
	expiredReports := []Report{{
		Text:      "expired report",
		Timestamp: time.Now().Add(-reportsBufferExpiration).Unix(),
	}}

	srv.addReportsToBuffer(expiredReports)

	test.Equal(t, srv.reportsBuffer, expiredReports)

	// adding fresh reports should remove the expired reports from the buffer
	newReports := []Report{{Text: "new report 1", Timestamp: time.Now().Unix()}}
	srv.addReportsToBuffer(newReports)

	test.Equal(t, srv.reportsBuffer, newReports)

	// adding more fresh reports shouldn't remove the other fresh reports from the buffer
	newReports2 := []Report{{Text: "new report 2", Timestamp: time.Now().Unix()}}
	srv.addReportsToBuffer(newReports2)

	test.Equal(t, srv.reportsBuffer, append(newReports, newReports2...))
}
