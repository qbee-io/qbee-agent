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

	return agent.Run(ctx)
}

// StartWithAutoUpdate starts the agent with auto-update functionality.
func StartWithAutoUpdate(ctx context.Context, cfg *Config) error {
	return Start(ctx, cfg)
}
