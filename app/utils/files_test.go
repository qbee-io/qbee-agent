package utils

import (
	"context"
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
