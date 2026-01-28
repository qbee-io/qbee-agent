// Copyright 2026 qbee.io
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
	"testing"

	"go.qbee.io/agent/app/configuration"
	"go.qbee.io/agent/app/utils/assert"
	"go.qbee.io/agent/app/utils/runner"
)

func Test_AddDeleteUsers(t *testing.T) {

	for _, tt := range privilegeTest {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := runner.New(t)

			if tt.unprivileged {
				r = r.WithUnprivileged()
			}
			testuser := configuration.User{
				Username: "testuser1",
				Action:   configuration.UserAdd,
			}

			reports := executeUsersBundle(r, []configuration.User{
				testuser,
			})

			expectedReports := []string{
				"[INFO] Successfully added user 'testuser1'",
			}

			assert.Equal(t, reports, expectedReports)
			r.MustExec("id", "-u", "testuser1")

			testuser.Action = configuration.UserRemove

			reports = executeUsersBundle(r, []configuration.User{
				testuser,
			})

			expectedReports = []string{
				"[INFO] Successfully removed user 'testuser1'",
			}

			assert.Equal(t, reports, expectedReports)

			_, err := r.Exec("id", "-u", "testuser1")
			assert.True(t, err != nil)
		})
	}
}

// executePasswordBundle is a helper method to quickly execute password bundle.
// On success, it returns a slice of produced reports.
func executeUsersBundle(r *runner.Runner, users []configuration.User) []string {
	config := configuration.CommittedConfig{
		Bundles: []string{configuration.BundleUsers},
		BundleData: configuration.BundleData{
			Users: &configuration.UsersBundle{
				Metadata: configuration.Metadata{Enabled: true},
				Users:    users,
			},
		},
	}

	reports, _ := configuration.ExecuteTestConfigInDocker(r, config)

	return reports
}
