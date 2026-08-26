package utils

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
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

	// Now, the goroutine count should be at maxGoroutines
	if !GoroutinesExhausted() {
		t.Fatalf("Expected GoroutinesExhausted to return true, but got false")
	}

	// Try to reserve one more slot, which should fail
	if reserveGoroutineSlot() {
		t.Fatalf("Expected reserveGoroutineSlot to return false when max goroutines reached, but got true")
	}
}

func TestExhaustGoRoutinesPipeReads(t *testing.T) {

	fifoPath := t.TempDir() + "/test_fifo"
	err := exhaustGoRoutines(t.Context(), fifoPath)
	if err != nil {
		t.Fatalf("Failed to exhaust goroutines: %v", err)
	}

	if got := atomic.LoadInt64(&goroutineCount); got != int64(maxGoroutines) {
		t.Fatalf("Expected goroutine count to stay at maxGoroutines (%d), got %d", maxGoroutines, got)
	}

	// try to read once more, which should fail to reserve a slot
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err = ReadFileContext(ctx, fifoPath)

	if err == nil {
		t.Fatalf("Expected ReadFileContext to fail due to exhausted goroutines, but it succeeded")
	}

	if got := atomic.LoadInt64(&goroutineCount); got != int64(maxGoroutines) {
		t.Fatalf("Expected goroutine count to stay at maxGoroutines (%d) after additional read, got %d", maxGoroutines, got)
	}
}

// ExhaustGoRoutines is a utility function to exhaust the goroutine slots for testing purposes.
func exhaustGoRoutines(ctx context.Context, fifoPath string) error {
	// Reset the goroutine count before the test
	atomic.StoreInt64(&goroutineCount, 0)

	// Ensure the FIFO does not already exist
	if _, err := os.Stat(fifoPath); err == nil {
		os.Remove(fifoPath)
	}

	// Create a named pipe (FIFO)
	if err := syscall.Mkfifo(fifoPath, 0666); err != nil {
		return err
	}

	// ReadFileContext for max go routines + 1 and wait for them to timeout,
	// then check that the goroutine count is still at maxGoroutines.
	var wg sync.WaitGroup
	for range maxGoroutines + 1 {
		wg.Go(func() {
			readCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			defer cancel()
			_, _ = ReadFileContext(readCtx, fifoPath)
		})
	}

	wg.Wait()
	return nil
}
