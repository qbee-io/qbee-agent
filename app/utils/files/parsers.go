package files

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.qbee.io/agent/app/utils"
)

// ForLinesInFile runs fn for every line in the provided filePath with a context.
func ForLinesInFile(ctx context.Context, timeOut time.Duration, filePath string, fn func(string) error) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeOut)
	defer cancel()

	return fileOperationWithContext(ctxWithTimeout, func() error {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("error opening file %s: %w", filePath, err)
		}
		defer func() { _ = file.Close() }()

		return utils.ForLines(file, fn)
	})
}
