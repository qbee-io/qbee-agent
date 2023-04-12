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

// RunOnce starts the agent and exits after the first run.
func RunOnce(ctx context.Context, cfg *Config) error {
	agent, err := New(cfg)
	if err != nil {
		return fmt.Errorf("error initializing the agent: %w", err)
	}

	agent.RunOnce(ctx, FullRun)

	agent.inProgress.Wait()

	return nil
}

// RunOnce starts the agent and exits after the first run.
func RunOnce(ctx context.Context, cfg *Config) error {
	agent, err := New(cfg)
	if err != nil {
		return fmt.Errorf("error initializing the agent: %w", err)
	}

	agent.RunOnce(ctx, FullRun)

	agent.inProgress.Wait()

	return nil
}
