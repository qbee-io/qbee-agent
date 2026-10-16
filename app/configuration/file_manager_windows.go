//go:build windows

package configuration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// openNoFollowFlag is not supported on Windows.
const openNoFollowFlag = 0

func chown(file *os.File, uid, gid int) error {
	// Windows does not have a direct equivalent to Unix file permissions.
	// This function is a no-op on Windows.
	return nil
}

func chownPath(path string, uid, gid int) error {
	// Windows does not have a direct equivalent to Unix file permissions.
	// This function is a no-op on Windows.
	return nil
}

func chownRoot(dir *os.Root, uid, gid int) error {
	// Windows does not have a direct equivalent to Unix file permissions.
	// This function is a no-op on Windows.
	return nil
}

// determineFileCreateData finds the closest existing path for dst. Ownership is not used on Windows.
func determineFileCreateData(dst string) (*fileCreateData, error) {
	if _, err := os.Stat(dst); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			parentDirPath := filepath.Dir(dst)

			if parentDirPath == dst {
				return &fileCreateData{path: dst}, nil
			}

			return determineFileCreateData(parentDirPath)
		}

		return nil, fmt.Errorf("cannot check file create data: %s - %w", dst, err)
	}

	return &fileCreateData{path: dst}, nil
}

// diskSpaceAvailable returns the number of free bytes on the filesystem containing path.
func diskSpaceAvailable(path string) (int64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("cannot check disk space: %s - %w", path, err)
	}

	var freeBytesAvailable uint64
	if err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, nil, nil); err != nil {
		return 0, fmt.Errorf("cannot check disk space: %s - %w", path, err)
	}

	return int64(freeBytesAvailable), nil
}

// verifyOwnedByAgent is a no-op on Windows, where Unix-style ownership does not apply.
func verifyOwnedByAgent(dir *os.Root) error {
	return nil
}

// renamePartialDownload renames oldName in partialDir to newName in dstRoot.
// partialDir is always a direct subdirectory of dstRoot.
func renamePartialDownload(dstRoot, partialDir *os.Root, oldName, newName string) error {
	partialDirName := filepath.Base(partialDir.Name())

	return dstRoot.Rename(filepath.Join(partialDirName, oldName), newName)
}
