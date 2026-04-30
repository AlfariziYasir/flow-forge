package service

import (
	"context"
	"fmt"
	"time"
)

type WaitAction struct{}

func NewWaitAction() *WaitAction {
	return &WaitAction{}
}

func (a *WaitAction) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	durationStr, ok := params["duration"].(string)
	if !ok {
		return nil, fmt.Errorf("missing 'duration' parameter")
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %v", err)
	}

	select {
	case <-time.After(duration):
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
