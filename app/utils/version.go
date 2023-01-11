package utils

import (
	"strconv"
	"strings"
)

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
	}

	return false
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
