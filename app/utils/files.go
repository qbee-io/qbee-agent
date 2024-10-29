package utils

import (
	"os"
	"os/user"
	"strconv"
)

// SetFileOwner sets the owner of the file at the specified path.
func SetFileOwner(path string, user *user.User) error {

	uid, err := strconv.ParseInt(user.Uid, 10, 32)
	if err != nil {
		return err
	}

	gid, err := strconv.ParseInt(user.Gid, 10, 32)
	if err != nil {
		return err
	}

	if err := os.Chown(path, int(uid), int(gid)); err != nil {
		return err
	}
	return nil
}
