package service_test

import (
	"context"
	"flowforge/internal/worker/service"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitAction_Success(t *testing.T) {
	action := service.NewWaitAction()
	params := map[string]any{
		"duration": "100ms",
	}

	start := time.Now()
	_, err := action.Execute(context.Background(), params)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.True(t, elapsed >= 100*time.Millisecond, "should wait at least 100ms")
}

func TestWaitAction_ContextCancelled(t *testing.T) {
	action := service.NewWaitAction()
	params := map[string]any{
		"duration": "1s",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := action.Execute(ctx, params)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestWaitAction_Validation(t *testing.T) {
	action := service.NewWaitAction()

	t.Run("Missing duration", func(t *testing.T) {
		_, err := action.Execute(context.Background(), map[string]any{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing 'duration' parameter")
	})

	t.Run("Invalid duration format", func(t *testing.T) {
		_, err := action.Execute(context.Background(), map[string]any{"duration": "invalid"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid duration")
	})
}
