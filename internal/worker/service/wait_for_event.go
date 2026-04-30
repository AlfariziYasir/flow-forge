package service

import (
	"context"
	"flowforge/pkg/errorx"
	"fmt"
)

type WaitForEventAction struct{}

func NewWaitForEventAction() *WaitForEventAction {
	return &WaitForEventAction{}
}

func (a *WaitForEventAction) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	eventName, _ := params["event_name"].(string)
	correlationKey, _ := params["correlation_key"].(string)

	if eventName == "" {
		return nil, fmt.Errorf("event_name is required for WAIT_FOR_EVENT action")
	}

	result := map[string]any{
		"waiting_for_event": eventName,
		"correlation_key":   correlationKey,
	}

	return result, errorx.ErrSuspendExecution
}
