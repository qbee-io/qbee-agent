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
	"strings"
	"testing"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_Password(t *testing.T) {
	r := runner.New(t)

	// assert that root is without a password
	rootLine := string(r.MustExec("sh", "-c", "cat /etc/shadow | grep 'root:'"))
	rootFields := strings.Split(rootLine, ":")
	assert.Equal(t, rootFields[1], "*")

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

	assert.Equal(t, reports, expectedReports)

	// check that root's password is updated
	rootLine = string(r.MustExec("sh", "-c", "cat /etc/shadow | grep 'root:'"))
	rootFields = strings.Split(rootLine, ":")
	assert.Equal(t, rootFields[1], newPassword)

	// check that executing the bundle again, won't make any changes
	reports = executePasswordBundle(r, userPasswords)
	assert.Empty(t, reports)

	// check that remaining records are untouched
	modifiedShadowWithoutRoot := r.MustExec("sh", "-c", "cat /etc/shadow | grep -v 'root:'")
	assert.Equal(t, string(modifiedShadowWithoutRoot), string(originalShadowWithoutRoot))
}

// executePasswordBundle is a helper method to quickly execute password bundle.
// On success, it returns a slice of produced reports.
func executePasswordBundle(r *runner.Runner, users []configuration.UserPassword) []string {
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
