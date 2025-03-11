package triggers

import (
	"context"
	"fmt"
	"github.com/live-labs/lokiactor/actions"
	"github.com/live-labs/lokiactor/config"
	"regexp"
)

type Trigger struct {
	Name        string
	Regex       *regexp.Regexp
	IgnoreRegex *regexp.Regexp

	Lines           int
	Action          actions.Action
	NextLinesAction actions.Action // if lines > 0
}

func New(ctx context.Context, cfg config.Trigger) (*Trigger, error) {
	re, err := regexp.Compile(cfg.Regex)
	if err != nil {
		return nil, err
	}
	ignoreRe, err := regexp.Compile(cfg.IgnoreRegex)
	if err != nil {
		return nil, err
	}

	action, err := actions.New(ctx, cfg.Action)

	if err != nil {
		return nil, fmt.Errorf("failed to create action %s: %w", cfg.Action.Type, err)
	}

	var nextLinesAction actions.Action
	if cfg.Lines > 0 {

		if cfg.NextLinesAction == nil {
			return nil, fmt.Errorf("next lines action is required for multiline trigger %s", cfg.Name)
		}

		nextLinesAction, err = actions.New(ctx, *cfg.NextLinesAction)
		if err != nil {
			return nil, fmt.Errorf("failed to create next lines action %s: %w", cfg.NextLinesAction.Type, err)
		}
	}

	return &Trigger{
		Name:            cfg.Name,
		Regex:           re,
		IgnoreRegex:     ignoreRe,
		Lines:           cfg.Lines,
		Action:          action,
		NextLinesAction: nextLinesAction,
	}, nil
}
