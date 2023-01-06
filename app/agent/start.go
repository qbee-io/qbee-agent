package agent

import (
	"context"
	"fmt"
)

// Start starts the agent.
func Start(ctx context.Context, cfg *Config) error {
	agent, err := New(cfg)
	if err != nil {
		return fmt.Errorf("error initializing the agent: %w", err)
	}

	_ = agent

	return fmt.Errorf("not implemented")
}

// StartWithAutoUpdate starts the agent with auto-update functionality.
func StartWithAutoUpdate(ctx context.Context, cfg *Config) error {
	return Start(ctx, cfg)
}
