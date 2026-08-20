package utils

import (
	"context"
	"io"
	"os"
	"sync/atomic"
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

func ReadFileContext(ctx context.Context, filePath string) ([]byte, error) {
	atomic.AddInt64(&goroutineCount, 1)
	defer atomic.AddInt64(&goroutineCount, -1)

	ch := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		data, err := os.ReadFile(filePath)
		if err != nil {
			errCh <- err
			return
		}
		ch <- data
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case data := <-ch:
		return data, nil
	}
}

// OpenFileContext opens a file with context cancellation support.
func OpenFileContext(ctx context.Context, filePath string) (io.Reader, error) {
	atomic.AddInt64(&goroutineCount, 1)

	ch := make(chan *os.File, 1)
	errCh := make(chan error, 1)

	go func() {
		defer atomic.AddInt64(&goroutineCount, -1)
		f, err := os.Open(filePath)
		if err != nil {
			errCh <- err
			return
		}
		ch <- f
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

func (cr *contextReader) Read(p []byte) (n int, err error) {
	select {
	case <-cr.ctx.Done():
		cr.f.Close()
		return 0, cr.ctx.Err()
	default:
	}
	return cr.f.Read(p)
}
