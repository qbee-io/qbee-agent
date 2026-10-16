package utils

import (
	"os"
	"regexp"
)

var hostnameSanitizationRe = regexp.MustCompile("[^a-zA-Z0-9-]")

// sanitizeHostname removes all characters from the hostname that are not letters, numbers, or hyphens.
// Based on https://man7.org/linux/man-pages/man5/hostname.5.html, but not enforcing all rules (length, format).
func sanitizeHostname(hostname string) string {
	return hostnameSanitizationRe.ReplaceAllString(hostname, "")
}

// Hostname returns the sanitized hostname of the current system, removing all characters that are not letters, numbers, or hyphens.
func Hostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}

	return sanitizeHostname(hostname), nil
}
