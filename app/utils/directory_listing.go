package utils

import (
	"fmt"
	"os"
)

// ListDirectory returns a list of files and directories under the provided dirPath.
func ListDirectory(dirPath string) ([]string, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, fmt.Errorf("error openning %s: %w", dirPath, err)
	}

	defer dir.Close()

	var dirNames []string
	if dirNames, err = dir.Readdirnames(-1); err != nil {
		return nil, fmt.Errorf("error listing contents of %s: %w", dirPath, err)
	}

	return dirNames, err
}
