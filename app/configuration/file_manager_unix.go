//go:build unix

package configuration

import (
	"os"
)

func chown(file *os.File, uid, gid int) error {
	return file.Chown(uid, gid)
}
