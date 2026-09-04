package files

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// goroutineCount tracks the number of active goroutines
var goroutineCount int64

// maxGoroutines defines the maximum number of concurrent goroutines allowed for file IO operations.
const maxGoroutines = 500

// GoroutinesExhausted reports whether the context file IO goroutine budget is exhausted.
func GoroutinesExhausted() bool {
	return atomic.LoadInt64(&goroutineCount) >= maxGoroutines
}

// SetGoroutineCountForTesting sets the goroutine count for testing purposes.
// This is only intended for use in tests.
func SetGoroutineCountForTesting(count int64) {
	atomic.StoreInt64(&goroutineCount, count)
}

// GetGoroutineCount returns the current goroutine count for testing purposes.
// This is only intended for use in tests.
func GetGoroutineCount() int64 {
	return atomic.LoadInt64(&goroutineCount)
}

// reserveGoroutineSlot attempts to reserve a slot for a new goroutine. It returns true if successful, false if the
// maximum number of goroutines has been reached. This method is safe for Time-of-Check to Time-of-Use (TOCTOU) race
// conditions. For loop ensures retries until the slot is successfully reserved or the maximum number of goroutines is reached.
func reserveGoroutineSlot() bool {
	for {
		current := atomic.LoadInt64(&goroutineCount)
		if current >= maxGoroutines {
			return false
		}

		if atomic.CompareAndSwapInt64(&goroutineCount, current, current+1) {
			return true
		}
	}
}

// releaseGoroutineSlot releases a previously reserved goroutine slot, decrementing the goroutine count unconditionally.
func releaseGoroutineSlot() {
	atomic.AddInt64(&goroutineCount, -1)
}

// KernelVirtualFSReadTimeout limits reads/opens on virtual kernel filesystems.
const KernelVirtualFSReadTimeout = 2 * time.Second

// Read reads the contents of a file with context cancellation support.
func Read(ctx context.Context, timeout time.Duration, maxBytes int64, filePath string) ([]byte, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var result []byte
	outerErr := fileOperationWithContext(ctxWithTimeout, func() error {
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()

		// If maxBytes is less than or equal to 0, read the entire file without limiting the number of bytes.
		if maxBytes <= 0 {
			result, err = io.ReadAll(file)
			return err
		}

		result, err = io.ReadAll(io.LimitReader(file, maxBytes))
		return err
	})

	if outerErr != nil {
		return nil, outerErr
	}
	return result, nil
}

// ReadAll reads the entire contents of a file with context cancellation support, up to a maximum number of bytes.
func ReadAll(ctx context.Context, timeout time.Duration, filePath string) ([]byte, error) {
	return Read(ctx, timeout, -1, filePath)
}

// Glob performs a filepath.Glob operation with context cancellation support.
func Glob(ctx context.Context, timeout time.Duration, pattern string) ([]string, error) {

	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var result []string

	outerErr := fileOperationWithContext(ctxWithTimeout, func() error {
		var err error
		result, err = filepath.Glob(pattern)
		if err != nil {
			return err
		}
		return nil
	})

	if outerErr != nil {
		return nil, outerErr
	}
	return result, nil
}

// ListDirectory lists files and directories in a directory with context cancellation support.
func ListDirectory(ctx context.Context, timeOut time.Duration, dirPath string) ([]string, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeOut)
	defer cancel()

	var result []string

	outerErr := fileOperationWithContext(ctxWithTimeout, func() error {
		dir, err := os.Open(dirPath)
		if err != nil {
			return err
		}

		defer func() { _ = dir.Close() }()

		result, err = dir.Readdirnames(-1)
		if err != nil {
			return err
		}
		return nil
	})

	if outerErr != nil {
		return nil, outerErr
	}
	return result, nil
}

// Stat performs an os.Stat operation with context cancellation support.
func Stat(ctx context.Context, timeOut time.Duration, path string) (os.FileInfo, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeOut)
	defer cancel()
	var result os.FileInfo

	outerErr := fileOperationWithContext(ctxWithTimeout, func() error {
		var err error
		result, err = os.Stat(path)
		if err != nil {
			return err
		}
		return nil
	})

	if outerErr != nil {
		return nil, outerErr
	}
	return result, nil
}

// fileOperationWithContext executes a file operation in a separate goroutine while respecting context cancellation.
// It ensures that the operation does not block the main goroutine and releases the goroutine slot after completion.
func fileOperationWithContext(ctx context.Context, operation func() error) error {
	if !reserveGoroutineSlot() {
		return fmt.Errorf("goroutine budget exhausted")
	}

	errCh := make(chan error, 1)

	go func() {
		defer func() {
			releaseGoroutineSlot()
		}()

		errCh <- operation()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
