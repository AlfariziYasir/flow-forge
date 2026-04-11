package services

import (
	"context"
	"encoding/json"
	"flowforge/internal/model"
)

type AIService interface {
	GenerateDAGFromText(ctx context.Context, prompt string) ([]model.StepDefinition, error)
	AnalyzeFailure(ctx context.Context, errorLog string) (string, error)
}

type aiService struct {
	// For a real implementation, you'd inject the Gemini client here.
	// client *genai.Client
}

func NewAIService() AIService {
	return &aiService{}
}

func (s *aiService) GenerateDAGFromText(ctx context.Context, prompt string) ([]model.StepDefinition, error) {
	// Mock implementation for MVP
	// In reality, this would send the prompt to Gemini with a strict JSON schema requirement
	mockDAG := `[
		{
			"id": "step_1",
			"action": "HTTP_CALL",
			"parameters": {"url": "https://api.example.com", "method": "GET"},
			"depends_on": [],
			"max_retries": 2
		},
		{
			"id": "step_2",
			"action": "SCRIPT",
			"parameters": {"script": "console.log('Success');"},
			"depends_on": ["step_1"],
			"max_retries": 0
		}
	]`
	
	var steps []model.StepDefinition
	err := json.Unmarshal([]byte(mockDAG), &steps)
	return steps, err
}

func (s *aiService) AnalyzeFailure(ctx context.Context, errorLog string) (string, error) {
	// Mock implementation for MVP
	instruction := "Analysis based on error: " + errorLog + "\n\n"
	instruction += "Suggested Fix: Check if the external service is reachable and if authentication headers are correct."
	return instruction, nil
}
