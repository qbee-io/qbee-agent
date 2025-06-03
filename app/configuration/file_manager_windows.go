//go:build windows

package configuration

import "os"

func chown(file *os.File, uid, gid int) error {
	// Windows does not have a direct equivalent to Unix file permissions.
	// This function is a no-op on Windows.
	return nil
}
