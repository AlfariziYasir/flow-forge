package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAIService_GenerateDAGFromText(t *testing.T) {
	svc := NewAIService()
	ctx := context.Background()

	res, err := svc.GenerateDAGFromText(ctx, "Generate a simple DAG")

	assert.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Equal(t, "step_1", res[0].ID)
}

func TestAIService_AnalyzeFailure(t *testing.T) {
	svc := NewAIService()
	ctx := context.Background()

	res, err := svc.AnalyzeFailure(ctx, "Some error log")

	assert.NoError(t, err)
	assert.NotEmpty(t, res)
	assert.Contains(t, res, "Analysis based on error")
}
