package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type ConditionAction struct{}

func (a *ConditionAction) Execute(ctx context.Context, params map[string]any) (map[string]any, error) {
	op, _ := params["operator"].(string)
	v1 := params["value_1"]
	v2 := params["value_2"]

	f1, ok1 := toFloat(v1)
	f2, ok2 := toFloat(v2)

	var conditionMet bool
	switch op {
	case "==":
		if ok1 && ok2 {
			conditionMet = f1 == f2
		} else {
			conditionMet = fmt.Sprintf("%v", v1) == fmt.Sprintf("%v", v2)
		}
	case "!=":
		if ok1 && ok2 {
			conditionMet = f1 != f2
		} else {
			conditionMet = fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2)
		}
	case ">":
		if ok1 && ok2 {
			conditionMet = f1 > f2
		} else {
			conditionMet = fmt.Sprintf("%v", v1) > fmt.Sprintf("%v", v2)
		}
	case ">=":
		if ok1 && ok2 {
			conditionMet = f1 >= f2
		} else {
			conditionMet = fmt.Sprintf("%v", v1) >= fmt.Sprintf("%v", v2)
		}
	case "<":
		if ok1 && ok2 {
			conditionMet = f1 < f2
		} else {
			conditionMet = fmt.Sprintf("%v", v1) < fmt.Sprintf("%v", v2)
		}
	case "<=":
		if ok1 && ok2 {
			conditionMet = f1 <= f2
		} else {
			conditionMet = fmt.Sprintf("%v", v1) <= fmt.Sprintf("%v", v2)
		}
	case "contains":
		conditionMet = strings.Contains(fmt.Sprintf("%v", v1), fmt.Sprintf("%v", v2))
	default:
		// Default sederhana jika user memberikan parameter "is_true": true
		condBool, ok := params["is_true"].(bool)
		conditionMet = ok && condBool
	}

	return map[string]any{
		"condition_met": conditionMet,
		"evaluated":     true,
	}, nil
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case float32:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		return f, err == nil
	}
	return 0, false
}
