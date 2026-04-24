package services

import (
	"context"
	"testing"

	"flowforge/pkg/logger"

	"github.com/stretchr/testify/assert"
)

func TestAIService_GenerateDAGFromText_NoAPIKey(t *testing.T) {
	log, _ := logger.New("info", "test", "1.0")
	svc, _ := NewAIService("", log)
	ctx := context.Background()

	res, err := svc.GenerateDAGFromText(ctx, "Generate a simple DAG")

	assert.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "not configured")
}

func TestAIService_AnalyzeFailure_NoAPIKey(t *testing.T) {
	log, _ := logger.New("info", "test", "1.0")
	svc, _ := NewAIService("", log)
	ctx := context.Background()

	res, err := svc.AnalyzeFailure(ctx, "Some error log")

	assert.Error(t, err)
	assert.Empty(t, res)
	assert.Contains(t, err.Error(), "not configured")
}