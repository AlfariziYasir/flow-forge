package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

type TransformAction struct{}

func NewTransformAction() *TransformAction {
	return &TransformAction{}
}

func (a *TransformAction) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	operation, _ := params["operation"].(string)
	input, _ := params["input"].(string)

	var output any
	switch operation {
	case "UPPERCASE":
		output = strings.ToUpper(input)
	case "LOWERCASE":
		output = strings.ToLower(input)
	case "TRIM":
		output = strings.TrimSpace(input)
	case "SPLIT":
		sep, _ := params["separator"].(string)
		if sep == "" {
			sep = ","
		}
		output = strings.Split(input, sep)
	case "EXTRACT_EMAIL":
		output = emailRegex.FindString(input)
	default:
		return nil, fmt.Errorf("unknown transform operation: %s", operation)
	}

	return map[string]any{"output": output}, nil
}
