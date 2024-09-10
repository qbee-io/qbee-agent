package utils

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func UnpackTar(tarPath string, destPath string) error {
	tarFile, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	switch filepath.Ext(tarPath) {
	case ".tar":
		return unpackTar(tarFile, destPath)
	case ".tar.gz":
		bz2Reader := bzip2.NewReader(tarFile)
		return unpackTar(bz2Reader, destPath)
	case ".tar.bz2":
		gzReader, err := gzip.NewReader(tarFile)
		if err != nil {
			return err
		}
		defer gzReader.Close()
		return unpackTar(gzReader, destPath)
	default:
		return fmt.Errorf("unsupported tar format: %s", tarPath)
	}
}

func unpackTar(reader io.Reader, destPath string) error {
	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(destPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			targetFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			defer targetFile.Close()

			if _, err := io.Copy(targetFile, tarReader); err != nil {
				return err
			}
		}
	}
	return nil
}
