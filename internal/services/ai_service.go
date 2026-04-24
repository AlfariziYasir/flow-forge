package services

import (
	"context"
	"encoding/json"
	"errors"
	"flowforge/internal/model"
	"flowforge/pkg/logger"

	"google.golang.org/genai"
	"go.uber.org/zap"
)

const (
	MaxRetries = 3
)

type AIService interface {
	GenerateDAGFromText(ctx context.Context, prompt string) ([]model.StepDefinition, error)
	AnalyzeFailure(ctx context.Context, errorLog string) (string, error)
}

type aiService struct {
	client  *genai.Client
	model   string
	log     *logger.Logger
}

func NewAIService(apiKey string, log *logger.Logger) (AIService, error) {
	if apiKey == "" {
		return &aiService{model: "", log: log}, nil
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, err
	}

	return &aiService{
		client: client,
		model:  "gemini-2.0-flash",
		log:    log,
	}, nil
}

func (s *aiService) GenerateDAGFromText(ctx context.Context, prompt string) ([]model.StepDefinition, error) {
	if s.client == nil {
		return nil, errors.New("AI service not configured: missing Gemini API key")
	}

	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		resp, err := s.client.Models.GenerateContent(ctx, s.model, []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					genai.NewPartFromText(dagSystemPrompt + "\n\nUser request: " + prompt),
				},
			},
		}, nil)
		if err != nil {
			s.log.Warn("failed to generate content", zap.Int("attempt", attempt+1), zap.Error(err))
			lastErr = err
			continue
		}

		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			lastErr = errors.New("empty response from AI")
			continue
		}

		result := resp.Candidates[0].Content.Parts[0].Text

		resultStr := extractJSON(result)

		var steps []model.StepDefinition
		if err := json.Unmarshal([]byte(resultStr), &steps); err != nil {
			s.log.Warn("failed to parse AI response", zap.Int("attempt", attempt+1), zap.Error(err))
			lastErr = err
			continue
		}

		return steps, nil
	}

	return nil, lastErr
}

func (s *aiService) AnalyzeFailure(ctx context.Context, errorLog string) (string, error) {
	if s.client == nil {
		return "", errors.New("AI service not configured: missing Gemini API key")
	}

	prompt := "Analyze the following error and provide a plain-English explanation with actionable fix recommendations:\n\n" + errorLog

	resp, err := s.client.Models.GenerateContent(ctx, s.model, []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				genai.NewPartFromText(prompt),
			},
		},
	}, nil)
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("empty response from AI")
	}

	return resp.Candidates[0].Content.Parts[0].Text, nil
}

var dagSystemPrompt = `You are a workflow automation expert. Convert the user's natural language description into a valid JSON array of workflow steps (DAG).

Each step must have:
- id: unique identifier (e.g., "step_1")
- action: one of HTTP, SCRIPT, DELAY, SWITCH, LOOP
- parameters: action-specific parameters
- depends_on: array of step IDs this step depends on
- max_retries: number of retry attempts

Example output for "When new user signs up, send welcome email and notify Slack":
[
  {
    "id": "step_1",
    "action": "HTTP",
    "parameters": {"url": "https://api.example.com/users", "method": "GET"},
    "depends_on": [],
    "max_retries": 2
  },
  {
    "id": "step_2",
    "action": "SCRIPT",
    "parameters": {"script": "return 'Welcome ' + input.name"},
    "depends_on": ["step_1"],
    "max_retries": 0
  },
  {
    "id": "step_3",
    "action": "HTTP",
    "parameters": {"url": "https://slack.com/api/chat.postMessage", "method": "POST", "body": {"text": "New user: " + input.name}},
    "depends_on": ["step_1"],
    "max_retries": 3
  }
]

Output only valid JSON array, no additional text.`

func extractJSON(s string) string {
	start := -1
	end := -1
	braceCount := 0

	for i, c := range s {
		if c == '[' && start == -1 {
			start = i
			braceCount = 1
			continue
		}
		if start != -1 {
			if c == '[' {
				braceCount++
			}
			if c == ']' {
				braceCount--
				if braceCount == 0 {
					end = i + 1
					break
				}
			}
		}
	}

	if start != -1 && end != -1 {
		return s[start:end]
	}

	start = -1
	end = -1
	braceCount = 0

	for i, c := range s {
		if c == '{' && start == -1 {
			start = i
			braceCount = 1
			continue
		}
		if start != -1 {
			if c == '{' {
				braceCount++
			}
			if c == '}' {
				braceCount--
				if braceCount == 0 {
					end = i + 1
					break
				}
			}
		}
	}

	if start != -1 && end != -1 {
		return s[start:end]
	}

	return s
}