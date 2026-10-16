package files

import (
	"sync/atomic"
	"testing"

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
