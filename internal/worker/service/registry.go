package service

import (
	"context"
	"fmt"
)

type Action interface {
	Execute(ctx context.Context, params map[string]any) (map[string]any, error)
}

type Registry struct {
	handlers map[string]Action
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Action),
	}
}

func (r *Registry) Registry(actionType string, handler Action) {
	r.handlers[actionType] = handler
}

func (r *Registry) Get(actionType string) (Action, error) {
	handlers, exists := r.handlers[actionType]
	if !exists {
		return nil, fmt.Errorf("action %s is not registered", actionType)
	}

	return handlers, nil
}
