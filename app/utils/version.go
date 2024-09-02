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
	"strconv"
	"strings"
)

// SemanticVersion represents a semantic version (major[.minor[.patch]])
type SemanticVersion [3]uint64

// IsNewerVersion returns true if "a" is newer than "b".
// Only semantic version is accepted (major[.minor[.patch]])
func IsNewerVersion(a, b string) bool {
	versionA := parseSemanticVersion(a)
	versionB := parseSemanticVersion(b)
	for i := 0; i < 3; i++ {
		if versionA[i] > versionB[i] {
			return true
		}
		if versionA[i] < versionB[i] {
			return false
		}
	}
	// versions are equal
	return false
}

// IsNewerVersionOrEqual returns true if "a" is newer or equal to "b".
func IsNewerVersionOrEqual(a, b string) bool {
	return !IsNewerVersion(b, a)
}

// parseSemanticVersion make a best-effort to parse a version string.
// For completely invalid strings, SemanticVersion{0, 0, 0} will be returned.
func parseSemanticVersion(versionString string) SemanticVersion {
	parts := strings.Split(strings.TrimPrefix(versionString, "v"), ".")

	version := SemanticVersion{0, 0, 0}

	partsCount := len(parts)
	if partsCount > len(version) {
		partsCount = len(version)
	}

	for i := 0; i < partsCount; i++ {
		value := make([]byte, 0, len(parts[i]))

		for _, ch := range parts[i] {
			if ch < '0' || ch > '9' {
				break
			}

			value = append(value, byte(ch))
		}

		intVersionPart, _ := strconv.Atoi(string(value))

		version[i] = uint64(intVersionPart)
	}

	return version
}
