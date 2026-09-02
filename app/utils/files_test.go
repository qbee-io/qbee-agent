package utils

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

func TestExhaustGoRoutines(t *testing.T) {
	// Reset the goroutine count before the test
	atomic.StoreInt64(&goroutineCount, 0)

	// Reserve the maximum number of goroutines
	for i := range maxGoroutines {
		if !reserveGoroutineSlot() {
			t.Fatalf("Failed to reserve goroutine slot at iteration %d", i)
		}
	}

	// Check that GoroutinesExhausted returns true when all slots are reserved
	assert.True(t, GoroutinesExhausted())

	// Try to reserve one more slot, which should fail
	assert.False(t, reserveGoroutineSlot())
}

func TestOpenFileWithContextClosesLateFile(t *testing.T) {
	fifoPath := t.TempDir() + "/test_fifo"
	atomic.StoreInt64(&goroutineCount, 0)
	assert.NoError(t, syscall.Mkfifo(fifoPath, 0666))

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	_, err := OpenFileWithContext(ctx, fifoPath)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))

	writer, err := os.OpenFile(fifoPath, os.O_WRONLY, 0)
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())

	assert.EventuallyTrue(t, func() bool {
		return atomic.LoadInt64(&goroutineCount) == 0
	}, time.Second)
}

func TestExhaustGoRoutinesPipeReads(t *testing.T) {

	fifoPath := t.TempDir() + "/test_fifo"

	// Reset the goroutine count before the test
	atomic.StoreInt64(&goroutineCount, 0)
	// Create a named pipe (FIFO)
	assert.NoError(t, syscall.Mkfifo(fifoPath, 0666))
	// ReadFileContext for max go routines + 1 and wait for them to timeout,
	// then check that the goroutine count is still at maxGoroutines.
	var wg sync.WaitGroup
	for range maxGoroutines + 1 {
		wg.Go(func() {
			readCtx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
			defer cancel()
			_, _ = ReadFileWithContext(readCtx, fifoPath)
		})
	}

	wg.Wait()

	assert.Equal(t, atomic.LoadInt64(&goroutineCount), int64(maxGoroutines))

	// try to read once more, which should fail to reserve a slot
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	_, err := ReadFileWithContext(ctx, fifoPath)
	assert.NotEmpty(t, err)
	assert.Equal(t, atomic.LoadInt64(&goroutineCount), int64(maxGoroutines))
}

func Test_ContextReaderDoesNotModifyBufferAfterCancellation(t *testing.T) {
	atomic.StoreInt64(&goroutineCount, 0)
	reader, writer, err := os.Pipe()
	assert.NoError(t, err)
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()

	ctx, cancel := context.WithCancel(t.Context())
	buffer := []byte("unchanged")
	result := make(chan error, 1)
	go func() {
		_, err := (&contextReader{ctx: ctx, f: reader}).Read(buffer)
		result <- err
	}()

	cancel()
	assert.True(t, errors.Is(<-result, context.Canceled))

	// The reader end of the pipe is closed by Read upon context cancellation,
	// so writing to it afterwards should fail rather than modify the buffer.
	_, err = writer.Write([]byte("changed"))
	assert.True(t, err != nil)
	assert.Equal(t, string(buffer), "unchanged")

	assert.EventuallyTrue(t, func() bool {
		return atomic.LoadInt64(&goroutineCount) == 0
	}, time.Second)
}

// Test timeouts for path-based file operations (ReadFile and ListDirectory) using named pipes (FIFOs)
func Test_FileTimeouts(t *testing.T) {

	tests := []struct {
		name string
		fn   func(ctx context.Context, path string) error
	}{
		{
			name: "ReadFileTimeout",
			fn: func(ctx context.Context, path string) error {
				_, err := ReadFileWithContext(ctx, path)
				return err
			}},
		{
			name: "ListDirectoryTimeout",
			fn: func(ctx context.Context, path string) error {
				_, err := ListDirectoryWithContext(ctx, path)
				return err
			}},
	}

	fifoPath := t.TempDir() + "/test_fifo"

	assert.NoError(t, syscall.Mkfifo(fifoPath, 0666))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atomic.StoreInt64(&goroutineCount, 0)

			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()

			err := tt.fn(ctx, fifoPath)
			assert.True(t, errors.Is(err, context.DeadlineExceeded))

			// Check that the goroutine count is 1 after the timeout occurred
			assert.True(t, atomic.LoadInt64(&goroutineCount) == 1)
		})
	}
}
