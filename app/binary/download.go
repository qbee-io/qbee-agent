package binary

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/qbee-io/qbee-agent/app/api"
)

const (
	Agent   = "agent"
	OpenVPN = "openvpn"
)

// Download and verify the latest binary version.
func Download(apiClient *api.Client, ctx context.Context, name, dstPath string) error {
	fp, err := os.CreateTemp(filepath.Dir(dstPath), filepath.Base(dstPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("cannot create temporary binary file: %w", err)
	}
	defer fp.Close()

	if err = fp.Chmod(nonExecutableFileMode); err != nil {
		return fmt.Errorf("cannot set permissions on temporary binary: %w", err)
	}

	tmpPath := fp.Name()

	// ensure temporary binary is removed in case of errors
	defer os.Remove(tmpPath)

	var metadata *Metadata
	if metadata, err = download(apiClient, ctx, name, fp); err != nil {
		return fmt.Errorf("cannot download update: %w", err)
	}

	if err = fp.Close(); err != nil {
		return fmt.Errorf("cannot close temporary binary file: %w", err)
	}

	if err = Verify(tmpPath, metadata); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	if err = os.Rename(tmpPath, dstPath); err != nil {
		return fmt.Errorf("cannot rename binary to %s: %w", dstPath, err)
	}

	return nil
}
