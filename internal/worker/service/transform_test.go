package service_test

import (
	"context"
	"flowforge/internal/worker/service"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransformAction_ExtractEmail(t *testing.T) {
	action := service.NewTransformAction()
	params := map[string]any{
		"operation": "EXTRACT_EMAIL",
		"input":     "Please contact us at support@flowforge.io for help.",
	}

	result, err := action.Execute(context.Background(), params)

	assert.NoError(t, err)
	assert.Equal(t, "support@flowforge.io", result["output"])
}

func TestTransformAction_Uppercase(t *testing.T) {
	action := service.NewTransformAction()
	params := map[string]any{
		"operation": "UPPERCASE",
		"input":     "hello world",
	}

	result, err := action.Execute(context.Background(), params)

	assert.NoError(t, err)
	assert.Equal(t, "HELLO WORLD", result["output"])
}
