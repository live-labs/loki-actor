package actions

import (
	"context"
	"fmt"
	"github.com/live-labs/lokiactor/config"
	"time"
)

const RFC3339_MILLI = "2006-01-02T15:04:05.000Z"

type Action interface {
	// Execute executes an action using the provided timestamp, message, and associated labels.
	Execute(ts time.Time, message string, labels map[string]string) error
}

// New creates a new action based on the provided configuration.
func New(ctx context.Context, cfg config.Action) (Action, error) {
	switch cfg.Type {
	case "slack":
		return NewSlackAction(ctx, cfg), nil
	case "cmd":
		return NewCMDAction(ctx, cfg), nil
	default:
		return nil, fmt.Errorf("unknown action type: %s", cfg.Type)
	}
}
