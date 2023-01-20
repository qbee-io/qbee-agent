package configuration_test

import (
	"strings"
	"testing"

	"github.com/qbee-io/qbee-agent/app/configuration"
	"github.com/qbee-io/qbee-agent/app/test"
)

func Test_Password(t *testing.T) {
	r := test.New(t)

	// assert that root is without a password
	rootLine := string(r.MustExec("sh", "-c", "cat /etc/shadow | grep 'root:'"))
	rootFields := strings.Split(rootLine, ":")
	test.Equal(t, rootFields[1], "*")

	originalShadowWithoutRoot := r.MustExec("sh", "-c", "cat /etc/shadow | grep -v 'root:'")

	// set new password for the user using the bundle
	newPassword := "$6$EMNbdq1ZkOAZSpFt$t6Ei4J11Ybip1A51sbBPTtQEVcFPPPUs"

	userPasswords := []configuration.UserPassword{
		{
			Username:     "root",
			PasswordHash: newPassword,
		},
		// password for unknown users won't be set
		{
			Username:     "unknownuser",
			PasswordHash: newPassword,
		},
	}

	// execute and verify that password change is reported
	reports := executePasswordBundle(r, userPasswords)
	expectedReports := []string{
		"[INFO] Password for user root successfully set.",
	}

	test.Equal(t, reports, expectedReports)

	// check that root's password is updated
	rootLine = string(r.MustExec("sh", "-c", "cat /etc/shadow | grep 'root:'"))
	rootFields = strings.Split(rootLine, ":")
	test.Equal(t, rootFields[1], newPassword)

	// check that executing the bundle again, won't make any changes
	reports = executePasswordBundle(r, userPasswords)
	test.Empty(t, reports)

	// check that remaining records are untouched
	modifiedShadowWithoutRoot := r.MustExec("sh", "-c", "cat /etc/shadow | grep -v 'root:'")
	test.Equal(t, string(modifiedShadowWithoutRoot), string(originalShadowWithoutRoot))
}

// executePasswordBundle is a helper method to quickly execute password bundle.
// On success, it returns a slice of produced reports.
func executePasswordBundle(r *test.Runner, users []configuration.UserPassword) []string {
	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundlePassword},
		BundleData: configuration.BundleData{
			Password: &configuration.PasswordBundle{
				Metadata: configuration.Metadata{Enabled: true},
				Users:    users,
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)

	return reports
}
