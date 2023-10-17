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

	"github.com/qbee-io/qbee-agent/app/inventory"
	"github.com/qbee-io/qbee-agent/app/utils"
)

// UsersBundle adds or removes users.
//
// Example payload:
//
//	{
//	 "items": [
//	   {
//	     "username": "test",
//	     "action": "remove"
//	   }
//	 ]
//	}
type UsersBundle struct {
	Metadata

	Users []User `json:"items"`
}

// UserAction defines what to do with a user.
type UserAction string

const (
	UserAdd    UserAction = "add"
	UserRemove UserAction = "remove"
)

// User defines a user to be modified in the system.
type User struct {
	Username string     `json:"username"`
	Action   UserAction `json:"action"`
}

// Execute users config on the system.
func (u UsersBundle) Execute(ctx context.Context, _ *Service) error {
	usersInventory, err := inventory.CollectUsersInventory()
	if err != nil {
		return err
	}

	for _, user := range u.Users {
		user.Username = resolveParameters(ctx, user.Username)

		userExists := usersInventory.GetUser(user.Username) != nil

		if user.Action == UserAdd && !userExists {
			_ = u.AddUser(ctx, user.Username)
		}

		if user.Action == UserRemove && userExists {
			_ = u.RemoveUser(ctx, user.Username)
		}
	}

	return nil
}

const (
	userAddCmd    = "/usr/sbin/useradd"
	userDeleteCmd = "/usr/sbin/userdel"
)

// AddUser to the system.
func (u UsersBundle) AddUser(ctx context.Context, username string) error {
	output, err := utils.RunCommand(ctx, []string{
		userAddCmd,
		"--comment", fmt.Sprintf("%s,,,,User added by qbee", username),
		"--create-home",
		"--shell", getShell(),
		username,
	})
	if err != nil {
		ReportError(ctx, output, "Unable to add user '%s'", username)

		return err
	}

	ReportInfo(ctx, output, "Successfully added user '%s'", username)

	return nil
}

// RemoveUser from the system along with its home directory and the user's mail spool.
func (u UsersBundle) RemoveUser(ctx context.Context, username string) error {
	if username == "root" {
		ReportWarning(ctx, nil, "Cannot remove administrative user '%s'", username)
		return fmt.Errorf("cannot delete root user")
	}

	output, err := utils.RunCommand(ctx, []string{
		userDeleteCmd,
		"--remove",
		username,
	})
	if err != nil {
		ReportError(ctx, output, "Unable to remove user '%s'", username)

		return err
	}

	ReportInfo(ctx, output, "Successfully removed user '%s'", username)

	return nil
}
