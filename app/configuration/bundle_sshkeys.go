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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.qbee.io/agent/app/inventory"
)

// SSHKeysBundle adds or removes authorized SSH keys for users.
//
// Example payload:
//
//	{
//	 "users": [
//	   {
//	     "username": "test",
//	     "userkeys": [
//	       "key1",
//	       "key2"
//	     ]
//	   }
//	 ]
//	}
type SSHKeysBundle struct {
	Metadata

	Users []SSHKey `json:"users"`
}

// SSHKey defines an SSH key to be added to a user.
type SSHKey struct {
	Username string   `json:"username"`
	Keys     []string `json:"userkeys"`
}

const (
	sshDirectory           = ".ssh"
	sshDirectoryPermission = 0700

	sshAuthorizedKeysFile           = "authorized_keys"
	sshAuthorizedKeysFilePermission = 0600
)

// Execute SSH Keys bundle on the system.
func (s SSHKeysBundle) Execute(ctx context.Context, _ *Service) error {
	usersInventory, err := inventory.CollectUsersInventory()
	if err != nil {
		return err
	}

	for _, user := range s.Users {
		user.Username = resolveParameters(ctx, user.Username)

		existingUser := usersInventory.GetUser(user.Username)

		// skip non-existing users
		if existingUser == nil {
			continue
		}

		var created bool

		if created, err = s.createAuthorizedKeysFile(existingUser, user.Keys); err != nil {
			ReportError(ctx, err, "Unable to write authorized_keys for user %s", user.Username)
			continue
		}

		if created {
			ReportInfo(ctx, nil, "Writing authorized_keys for user %s.", user.Username)
		}
	}

	return nil
}

// createAuthorizedKeysFile checks whether authorized_keys file exists and has the right content.
// If not, recreate it and return true.
func (s SSHKeysBundle) createAuthorizedKeysFile(user *inventory.User, keys []string) (bool, error) {
	authorizedKeysFilePath := filepath.Join(user.HomeDirectory, sshDirectory, sshAuthorizedKeysFile)

	buf := bytes.NewBufferString(strings.Join(keys, "\n") + "\n")

	// calculate expected file digest
	digest := sha256.New()
	if _, err := digest.Write(buf.Bytes()); err != nil {
		return false, fmt.Errorf("cannot calculate digest of the authorized_keys file: %w", err)
	}
	hexDigest := hex.EncodeToString(digest.Sum(nil))

	// check whether the file has correct contents

	fileMetadata := &FileMetadata{
		Tags: map[string]string{
			fileDigestSHA256Tag: hexDigest,
		},
	}

	fileReady, err := isFileReady(authorizedKeysFilePath, fileMetadata)
	if err != nil || fileReady {
		return false, err
	}

	fileCreateData, err := determineFileCreateData(authorizedKeysFilePath)
	if err != nil {
		return false, fmt.Errorf("error determining local fs data: %w", err)
	}

	// ensure .ssh directory exists with the right permissions
	if err = makeDirectories(authorizedKeysFilePath, sshDirectoryPermission, user.UID, user.UID); err != nil {
		return false, err
	}

	// re-create authorized_keys file
	var file *os.File
	if file, err = createFile(authorizedKeysFilePath, fileCreateData, sshAuthorizedKeysFilePermission, true); err != nil {
		return false, err
	}

	defer func() { _ = file.Close() }()

	if _, err = file.Write(buf.Bytes()); err != nil {
		return false, fmt.Errorf("cannot write file %s: %w", authorizedKeysFilePath, err)
	}

	return true, nil
}
