package configuration

import (
	"context"
	"strings"

	"github.com/qbee-io/qbee-agent/app/utils"
)

// CheckPreCondition checks if the provided pre-condition is met.
func CheckPreCondition(ctx context.Context, preCondition string) bool {
	preCondition = resolveParameters(ctx, preCondition)

	preCondition = strings.TrimSpace(preCondition)

	if preCondition == "" {
		return true
	}

	// return with no error when pre-condition fails
	if _, err := utils.RunCommand(ctx, []string{getShell(), "-c", preCondition}); err != nil {
		return false
	}

	return true
}
