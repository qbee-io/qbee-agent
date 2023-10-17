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

package configuration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/qbee-io/qbee-agent/app/inventory"
)

// PasswordBundle bundle sets passwords for existing users.
//
// Example payload:
//
//	{
//	 "users": [
//	   {
//	     "username": "piotr",
//	     "passwordhash": "$6$EMNbdq1ZkOAZSpFt$t6Ei4J11Ybip1A51sbBPTtQEVcFPPPUs.Q9nle4FenvrId4fLr8douwE3lbgWZGK.LIPeVmmFrTxYJ0QoYkFT."
//	   }
//	 ]
//	}
type PasswordBundle struct {
	Metadata

	Users []UserPassword `json:"users"`
}

type UserPassword struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordhash"`
}

const secondsInADay = 60 * 60 * 24
const shadowFileMode = 0640

func (p PasswordBundle) Execute(ctx context.Context, service *Service) error {
	// convert user passwords to a map for quick lookup
	passwordMap := make(map[string]string)
	for _, user := range p.Users {
		username := resolveParameters(ctx, user.Username)
		passwordMap[username] = user.PasswordHash
	}

	// get current shadow file
	data, err := os.ReadFile(inventory.ShadowFilePath)
	if err != nil {
		ReportError(ctx, err, "Unable to manage passwords.")
		return err
	}

	// process all the lines and check if users have the right passwords
	currentLines := strings.Split(string(data), "\n")
	outputLines := make([]string, len(currentLines))
	modifiedUsers := make([]string, 0)

	for i, line := range currentLines {
		fields := strings.Split(line, ":")
		expectedPassword, ok := passwordMap[fields[0]]

		// copy without changes if user doesn't have a defined password configuration or the password is already correct
		if !ok || fields[1] == expectedPassword {
			outputLines[i] = line
			continue
		}

		// set expected password and reset its age
		fields[1] = expectedPassword
		fields[2] = fmt.Sprintf("%d", time.Now().Unix()/secondsInADay)
		outputLines[i] = strings.Join(fields, ":")
		modifiedUsers = append(modifiedUsers, fields[0])
	}

	// no changes needed
	if len(modifiedUsers) == 0 {
		return nil
	}

	// changes were made, so we need to write the new shadow file
	err = os.WriteFile(inventory.ShadowFilePath, []byte(strings.Join(outputLines, "\n")), shadowFileMode)
	if err != nil {
		ReportError(ctx, err, "Error setting passwords for users.")
		return err
	}

	// and report users for which we changed the password
	for _, user := range modifiedUsers {
		ReportInfo(ctx, nil, "Password for user %s successfully set.", user)
	}

	return nil
}
