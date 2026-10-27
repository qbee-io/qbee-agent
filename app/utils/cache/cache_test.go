package cache

import (
	"testing"
	"time"

	"go.qbee.io/agent/app/utils/assert"
)

func TestSetWithoutExpiry(t *testing.T) {
	t.Cleanup(Clear)

	SetWithoutExpiry("key", "value")

	data, ok := Get("key")
	assert.True(t, ok)
	assert.Equal(t, data, any("value"))
}

func TestSetExpired(t *testing.T) {
	t.Cleanup(Clear)

	Set("key", "value", -time.Second)

	_, ok := Get("key")
	assert.False(t, ok)
}
