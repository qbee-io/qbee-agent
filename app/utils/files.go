package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
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

// reserveGoroutineSlot attempts to reserve a slot for a new goroutine. It returns true if successful, false if the maximum number of goroutines has been reached.
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

// KernelVirtualFSReadTimeout limits reads/opens on virtual kernel filesystems.
const KernelVirtualFSReadTimeout = 2 * time.Second

// ReadFileWithContext reads the contents of a file with context cancellation support.
func ReadFileWithContext(ctx context.Context, filePath string) ([]byte, error) {
	reader, err := OpenFileWithContext(ctx, filePath)
	if err != nil {
		return nil, err
	}

	if closer, ok := reader.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}

	return io.ReadAll(reader)
}

// OpenFileWithContext opens a file with context cancellation support.
func OpenFileWithContext(ctx context.Context, filePath string) (io.Reader, error) {
	if !reserveGoroutineSlot() {
		return nil, fmt.Errorf("goroutine budget exhausted")
	}

ch := make(chan *os.File)
	errCh := make(chan error, 1)

	go func() {
		f, err := os.Open(filePath)
		if err != nil {
			atomic.AddInt64(&goroutineCount, -1)
			errCh <- err
			return
		}

		select {
		case <-ctx.Done():
			_ = f.Close()
			atomic.AddInt64(&goroutineCount, -1)
		case ch <- f:
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case f := <-ch:
		return &contextReader{
			ctx:  ctx,
			f:    f,
			done: func() { atomic.AddInt64(&goroutineCount, -1) },
		}, nil
	}
}

type contextReader struct {
	ctx  context.Context
	f    *os.File
	done func()
	once sync.Once
}

func (cr *contextReader) markDone() {
	cr.once.Do(cr.done)
}

func (cr *contextReader) Read(p []byte) (n int, err error) {
	select {
	case <-cr.ctx.Done():
		_ = cr.f.Close()
		cr.markDone()
		return 0, cr.ctx.Err()
	default:
	}

	n, err = cr.f.Read(p)
	if err != nil {
		cr.markDone()
	}

	return n, err
}

func (cr *contextReader) Close() error {
	err := cr.f.Close()
	cr.markDone()
	return err
}

func GlobWithContext(ctx context.Context, pattern string) ([]string, error) {
	if !reserveGoroutineSlot() {
		return nil, fmt.Errorf("goroutine budget exhausted")
	}

	ch := make(chan []string, 1)
	errCh := make(chan error, 1)

go func() {
		defer atomic.AddInt64(&goroutineCount, -1)

		matches, err := filepath.Glob(pattern)
		if err != nil {
			errCh <- err
			return
		}

		select {
		case <-ctx.Done():
		case ch <- matches:
		}
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
// This prevents blocking reads on virtual filesystems like procfs.
func ListDirectoryWithContext(ctx context.Context, dirPath string) ([]string, error) {
	if !reserveGoroutineSlot() {
		return nil, fmt.Errorf("goroutine budget exhausted")
	}

	ch := make(chan []string, 1)
	errCh := make(chan error, 1)

	go func() {
		dir, err := os.Open(dirPath)
		if err != nil {
			atomic.AddInt64(&goroutineCount, -1)
			errCh <- fmt.Errorf("error opening %s: %w", dirPath, err)
			return
		}

		defer func() { _ = dir.Close() }()

		dirNames, err := dir.Readdirnames(-1)
		if err != nil {
			atomic.AddInt64(&goroutineCount, -1)
			errCh <- fmt.Errorf("error listing contents of %s: %w", dirPath, err)
			return
		}

		select {
		case <-ctx.Done():
			atomic.AddInt64(&goroutineCount, -1)
		case ch <- dirNames:
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case names := <-ch:
		atomic.AddInt64(&goroutineCount, -1)
		return names, nil
	}
}
