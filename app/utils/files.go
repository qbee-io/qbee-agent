package utils

import "os"

// WriteFileSync writes data to a file named by filename and syncs to disk.
func WriteFileSync(name string, data []byte, perm os.FileMode) error {
	var err error
	var f *os.File

	f, err = os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	defer func() {
		if err1 := f.Close(); err1 != nil && err == nil {
			err = err1
		}
	}()

	if _, err = f.Write(data); err != nil {
		return err
	}

	if err = f.Sync(); err != nil {
		return err
	}

	return err
}
