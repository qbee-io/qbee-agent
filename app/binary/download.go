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

package binary

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.qbee.io/agent/app/api"
)

// Binaries supported for automatic distribution and verification.
const (
	Agent   = "agent"
	OpenVPN = "openvpn"
)

// Download and verify the latest binary version.
func Download(ctx context.Context, apiClient *api.Client, name, dstPath string) error {
	fp, err := os.CreateTemp(filepath.Dir(dstPath), filepath.Base(dstPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("cannot create temporary binary file: %w", err)
	}
	defer fp.Close()

	if err = fp.Chmod(nonExecutableFileMode); err != nil {
		return fmt.Errorf("cannot set permissions on temporary binary: %w", err)
	}

	tmpPath := fp.Name()

	// ensure temporary binary is removed in case of errors
	defer os.Remove(tmpPath)

	var metadata *Metadata
	if metadata, err = download(ctx, apiClient, name, fp); err != nil {
		return fmt.Errorf("cannot download update: %w", err)
	}

	if err = fp.Close(); err != nil {
		return fmt.Errorf("cannot close temporary binary file: %w", err)
	}

	if err = Verify(tmpPath, metadata); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	if err = os.Rename(tmpPath, dstPath); err != nil {
		return fmt.Errorf("cannot rename binary to %s: %w", dstPath, err)
	}

	return nil
}
