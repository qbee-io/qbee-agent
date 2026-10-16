//go:build unix

package configuration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// openNoFollowFlag makes os.OpenFile reject a symlinked final path component.
const openNoFollowFlag = unix.O_NOFOLLOW

func chown(file *os.File, uid, gid int) error {
	return file.Chown(uid, gid)
}

func chownPath(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}

func chownRoot(dir *os.Root, uid, gid int) error {
	return dir.Chown(".", uid, gid)
}

// determineFileCreateData detects uid and gid for the path.
func determineFileCreateData(dst string) (*fileCreateData, error) {

	var fileStat unix.Stat_t
	if err := unix.Stat(dst, &fileStat); err != nil {
		// if path doesn't exist, try to determine owner of the parent directory
		if errors.Is(err, fs.ErrNotExist) {
			parentDirPath := filepath.Dir(dst)

			if parentDirPath == dst {
				// this should never happen, but in case it does, use the process uid/gid
				return &fileCreateData{
					path: dst,
					uid:  os.Geteuid(),
					gid:  os.Getgid(),
				}, nil
			}

			return determineFileCreateData(parentDirPath)
		}

		return nil, fmt.Errorf("cannot check file create data: %s - %w", dst, err)
	}

	// if file exists, use its uid/gid
	return &fileCreateData{
		uid:  int(fileStat.Uid),
		gid:  int(fileStat.Gid),
		path: dst,
	}, nil
}

// diskSpaceAvailable returns the number of free bytes on the filesystem containing path.
func diskSpaceAvailable(path string) (int64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("cannot check disk space: %s - %w", path, err)
	}

	return int64(stat.Bavail) * int64(stat.Bsize), nil
}

// verifyOwnedByAgent fails if dir isn't owned by the agent's effective user, e.g. because it was
// swapped out right after being created.
func verifyOwnedByAgent(dir *os.Root) error {
	dirFile, err := dir.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = dirFile.Close() }()

	var stat unix.Stat_t
	if err = unix.Fstat(int(dirFile.Fd()), &stat); err != nil {
		return err
	}

	if int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("not owned by the agent")
	}

	return nil
}

// renamePartialDownload renames oldName in partialDir to newName in dstRoot.
//
// The rename is issued with renameat(2) against the already-open directory descriptors, so neither
// directory is resolved by name at rename time (CWE-59, CWE-367).
func renamePartialDownload(dstRoot, partialDir *os.Root, oldName, newName string) error {
	partialDirFile, err := partialDir.Open(".")
	if err != nil {
		return fmt.Errorf("error opening partial download directory: %w", err)
	}
	defer func() { _ = partialDirFile.Close() }()

	dstDirFile, err := dstRoot.Open(".")
	if err != nil {
		return fmt.Errorf("error opening destination directory: %w", err)
	}
	defer func() { _ = dstDirFile.Close() }()

	return unix.Renameat(int(partialDirFile.Fd()), oldName, int(dstDirFile.Fd()), newName)
}
