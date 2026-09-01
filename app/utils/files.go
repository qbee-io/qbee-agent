package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

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

// releaseGoroutineSlot releases a previously reserved goroutine slot. It decrements the goroutine count if the error is nil
// or not a context deadline exceeded error (context.DeadlineExceeded).
func releaseGoroutineSlot() {
	atomic.AddInt64(&goroutineCount, -1)
}

// KernelVirtualFSReadTimeout limits reads/opens on virtual kernel filesystems.
const KernelVirtualFSReadTimeout = 2 * time.Second

// ReadFileWithContext reads the contents of a file with context cancellation support.
func ReadFileWithContext(ctx context.Context, filePath string) ([]byte, error) {
	reader, err := OpenFileWithContext(ctx, filePath)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = reader.(*contextReader).Close()
	}()

	return reader.(*contextReader).ReadAll()
}

// OpenFileWithContext opens a file with context cancellation support.
func OpenFileWithContext(ctx context.Context, filePath string) (io.Reader, error) {
	if !reserveGoroutineSlot() {
		return nil, fmt.Errorf("goroutine budget exhausted")
	}

	ch := make(chan *os.File)
	errCh := make(chan error, 1)

	go func() {
		defer func() {
			releaseGoroutineSlot()
		}()

		f, err := os.Open(filePath)
		if err != nil {
			errCh <- err
			return
		}

		select {
		case ch <- f:
		case <-ctx.Done():
			_ = f.Close()
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case f := <-ch:
		return &contextReader{
			ctx: ctx,
			f:   f,
		}, nil
	}
}

type contextReader struct {
	ctx context.Context
	f   *os.File
}

// Read reads from the contextReader, respecting context cancellation. If the context is done, it closes the underlying file and returns an error.
func (cr *contextReader) Read(p []byte) (int, error) {
	if !reserveGoroutineSlot() {
		return 0, fmt.Errorf("goroutine budget exhausted")
	}

	errChan := make(chan error, 1)
	ch := make(chan int, 1)
	go func() {
		defer func() {
			releaseGoroutineSlot()
		}()

		n, err := cr.f.Read(p)
		if err != nil {
			errChan <- err
			return
		}
		ch <- n
	}()

	select {
	case <-cr.ctx.Done():
		return 0, cr.ctx.Err()
	case err := <-errChan:
		return 0, err
	case n := <-ch:
		return n, nil
	}
}

// ReadAll reads all data from the contextReader, respecting context cancellation. If the context is done, it closes the underlying file and returns an error.
func (cr *contextReader) ReadAll() ([]byte, error) {
	var result []byte
	buf := make([]byte, 4096)
	for {
		n, err := cr.Read(buf)
		if n > 0 {
			result = append(result, buf[:n]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return result, nil
			}
			return result, err
		}
	}
}

// Close closes the contextReader, releasing the underlying file and goroutine slot.
func (cr *contextReader) Close() error {
	return cr.f.Close()
}

// GlobWithContext performs a filepath.Glob operation with context cancellation support.
// This function performs a filepath.Glob operation while respecting context cancellation
// which prevents blocking on slow or unresponsive filesystems.
func GlobWithContext(ctx context.Context, pattern string) ([]string, error) {
	if !reserveGoroutineSlot() {
		return nil, fmt.Errorf("goroutine budget exhausted")
	}

	ch := make(chan []string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer func() {
			releaseGoroutineSlot()
		}()

		matches, err := filepath.Glob(pattern)
		if err != nil {
			errCh <- err
			return
		}

		ch <- matches
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case matches := <-ch:
		return matches, nil
	}
}

// ListDirectoryWithContext lists files and directories in a directory with context cancellation support.
// This function lists the contents of a directory while respecting context cancellation, preventing
// blocking on slow or unresponsive filesystems.
func ListDirectoryWithContext(ctx context.Context, dirPath string) ([]string, error) {
	if !reserveGoroutineSlot() {
		return nil, fmt.Errorf("goroutine budget exhausted")
	}

	ch := make(chan []string, 1)
	errCh := make(chan error, 1)

	go func() {
		defer func() {
			releaseGoroutineSlot()
		}()

		dir, err := os.Open(dirPath)
		if err != nil {
			errCh <- err
			return
		}

		defer func() { _ = dir.Close() }()

		dirNames, err := dir.Readdirnames(-1)
		if err != nil {
			errCh <- err
			return
		}
		ch <- dirNames
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case names := <-ch:
		return names, nil
	}
}
