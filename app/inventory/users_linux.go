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

//go:build linux

package inventory

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"go.qbee.io/agent/app/utils"
)

// Paths to standard passwd and shadow files.
const (
	PasswdFilePath = "/etc/passwd"
	ShadowFilePath = "/etc/shadow"
)

// Password algorithms recognized by Qbee.
const (
	PasswordAlgorithmMD5      = 1
	PasswordAlgorithmBcrypt   = 2
	PasswordAlgorithmSHA1     = 4
	PasswordAlgorithmSHA256   = 5
	PasswordAlgorithmSHA512   = 6
	PasswordAlgorithmYesCrypt = 7
)

var shadowAlgorithms = map[string]int{
	"1":    PasswordAlgorithmMD5,
	"2":    PasswordAlgorithmBcrypt,
	"2a":   PasswordAlgorithmBcrypt,
	"2b":   PasswordAlgorithmBcrypt,
	"2x":   PasswordAlgorithmBcrypt,
	"2y":   PasswordAlgorithmBcrypt,
	"sha1": PasswordAlgorithmSHA1,
	"5":    PasswordAlgorithmSHA256,
	"6":    PasswordAlgorithmSHA512,
	"7":    PasswordAlgorithmYesCrypt,
	"y":    PasswordAlgorithmYesCrypt,
}

// CollectUsersInventory returns populated Users inventory based on current system status.
func CollectUsersInventory() (*Users, error) {
	users, err := GetUsersFromPasswd(PasswdFilePath, ShadowFilePath)
	if err != nil {
		return nil, err
	}

	usersInventory := &Users{
		Users: users,
	}

	return usersInventory, nil
}

// GetUsersFromPasswd returns users based on passwd file.
func GetUsersFromPasswd(passwdFilePath, shadowFilePath string) ([]User, error) {
	// get mapping of username -> User (with populated password fields)
	usersPasswords, err := getUsersFromShadow(shadowFilePath)
	if err != nil {
		return nil, err
	}

	users := make([]User, 0)
	err = utils.ForLinesInFile(passwdFilePath, func(line string) error {
		fields := strings.Split(line, ":")

		if len(fields) < 7 {
			return nil
		}

		var uid, gid int
		if uid, err = strconv.Atoi(fields[2]); err != nil {
			return fmt.Errorf("invalid UID")
		}

		if gid, err = strconv.Atoi(fields[3]); err != nil {
			return fmt.Errorf("invalid GID")
		}

		user := User{
			Name:          fields[0],
			UID:           uid,
			GID:           gid,
			GECOS:         fields[4],
			HomeDirectory: fields[5],
			Shell:         fields[6],
			HasPassword:   "no",
		}

		// check if password is specified in the passwd file
		password := fields[1]
		if password != "x" && password != "*" && password != "" {
			user.HasPassword = "yes"
		}

		// passwords from shadow take precedence
		userPassword, ok := usersPasswords[user.Name]
		if ok {
			user.HasPassword = userPassword.HasPassword
			user.PasswordAlgorithm = userPassword.PasswordAlgorithm
			user.PasswordAge = userPassword.PasswordAge
		}

		users = append(users, user)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return users, nil
}

// getUsersFromShadow returns map of users with password related fields populated.
func getUsersFromShadow(filePath string) (map[string]User, error) {
	users := make(map[string]User)

	err := utils.ForLinesInFile(filePath, func(line string) error {
		fields := strings.Split(line, ":")

		if len(fields) < 3 {
			return nil
		}

		// skip invalid passwords
		passwordFields := strings.Split(fields[1], "$")
		if len(passwordFields) == 1 {
			return nil
		}

		age, err := strconv.Atoi(fields[2])
		if err != nil {
			return fmt.Errorf("invalid passowrd age")
		}

		users[fields[0]] = User{
			HasPassword:       "yes",
			PasswordAlgorithm: shadowAlgorithms[passwordFields[1]],
			PasswordAge:       age,
		}

		return nil
	})
	if err != nil {
		// we should be able to continue on systems without shadow file
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
			return users, nil
		}

		return nil, err
	}

	return users, nil
}
