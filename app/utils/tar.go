// Copyright 2024 qbee.io
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
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// UnpackTar unpacks a tar archive to a destination directory.
func UnpackTar(tarPath string, destPath string) error {
	tarFile, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer func() { _ = tarFile.Close() }()

	switch GetTarExtension(tarPath) {
	case "tar":
		return unpackTar(tarFile, destPath)
	case "tar.bz2":
		bz2Reader := bzip2.NewReader(tarFile)
		return unpackTar(bz2Reader, destPath)
	case "tar.gz":
		gzReader, err := gzip.NewReader(tarFile)
		if err != nil {
			return err
		}
		defer func() { _ = gzReader.Close() }()
		return unpackTar(gzReader, destPath)
	default:
		return fmt.Errorf("unsupported tar format: %s", tarPath)
	}
}

// IsSupportedTarExtension returns true if the tarPath has a supported extension.
func IsSupportedTarExtension(tarPath string) bool {
	switch GetTarExtension(tarPath) {
	case "tar", "tar.gz", "tar.bz2":
		return true
	default:
		return false
	}
}

// GetTarExtension returns the extension of a tar file.
func GetTarExtension(tarPath string) string {
	basename := filepath.Base(tarPath)
	parts := strings.Split(basename, ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[1:], ".")
}

// unpackTar unpacks a tar archive to a destination directory.
func unpackTar(reader io.Reader, destPath string) error {
	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Clean(filepath.Join(destPath, header.Name))
		destdirClean := filepath.Clean(destPath)

		if !strings.HasPrefix(targetPath, destdirClean) {
			return fmt.Errorf("tar entry %s is outside of destination directory %s", targetPath, destdirClean)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			targetFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			defer func() { _ = targetFile.Close() }()

			if _, err := io.Copy(targetFile, tarReader); err != nil {
				return err
			}
		}
	}
	return nil
}
