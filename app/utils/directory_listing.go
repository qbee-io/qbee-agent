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

package utils

import (
	"fmt"
	"os"
)

// ListDirectory returns a list of files and directories under the provided dirPath.
func ListDirectory(dirPath string) ([]string, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, fmt.Errorf("error openning %s: %w", dirPath, err)
	}

	defer dir.Close()

	var dirNames []string
	if dirNames, err = dir.Readdirnames(-1); err != nil {
		return nil, fmt.Errorf("error listing contents of %s: %w", dirPath, err)
	}

	return dirNames, err
}
