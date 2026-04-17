package service

import (
	"context"
	"fmt"
	"strings"
)

type ConditionAction struct{}

func (a *ConditionAction) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	op, _ := params["operator"].(string)
	v1 := params["value_1"]
	v2 := params["value_2"]

	var conditionMet bool
	switch op {
	case "==":
		conditionMet = fmt.Sprintf("%v", v1) == fmt.Sprintf("%v", v2)
	case "!=":
		conditionMet = fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2)
	case ">":
		conditionMet = fmt.Sprintf("%v", v1) > fmt.Sprintf("%v", v2)
	case ">=":
		conditionMet = fmt.Sprintf("%v", v1) >= fmt.Sprintf("%v", v2)
	case "<":
		conditionMet = fmt.Sprintf("%v", v1) < fmt.Sprintf("%v", v2)
	case "<=":
		conditionMet = fmt.Sprintf("%v", v1) <= fmt.Sprintf("%v", v2)
	case "contains":
		conditionMet = strings.Contains(fmt.Sprintf("%v", v1), fmt.Sprintf("%v", v2))
	default:
		// Default sederhana jika user memberikan parameter "is_true": true
		condBool, ok := params["is_true"].(bool)
		conditionMet = ok && condBool
	}

	return map[string]any{
		"condition_met": conditionMet,
		"evaluted":      true,
	}, nil
}
