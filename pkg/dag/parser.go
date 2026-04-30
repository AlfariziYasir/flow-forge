package dag

import (
	"errors"
	"flowforge/internal/model"
	"flowforge/pkg/errorx"
	"fmt"
)

func BuildExecutionPlan(steps []model.StepDefinition) ([][]model.StepDefinition, error) {
	if len(steps) == 0 {
		return nil, errorx.NewError(errorx.ErrTypeValidation, "workflow cannot empty", errors.New("workflow cannot empty"))
	}

	adjList := make(map[string][]string)
	inDegree := make(map[string]int)
	stepMap := make(map[string]model.StepDefinition)

	for _, step := range steps {
		if _, exists := stepMap[step.ID]; exists {
			return nil, errorx.NewError(errorx.ErrTypeValidation, "duplicate step id found", fmt.Errorf("duplicate step id found: %s", step.ID))
		}
		stepMap[step.ID] = step
		inDegree[step.ID] = 0
		adjList[step.ID] = []string{}
	}

	for _, step := range steps {
		for _, depID := range step.DependsOn {
			if _, exists := stepMap[depID]; !exists {
				return nil, errorx.NewError(errorx.ErrTypeValidation, "step depends on a non-existent step", fmt.Errorf("step '%s' depends on a non-existent step: '%s'", step.ID, depID))
			}

			adjList[depID] = append(adjList[depID], step.ID)
			inDegree[step.ID]++
		}
	}

	var executionLayers [][]model.StepDefinition
	var currentQueue []string
	for id, deg := range inDegree {
		if deg == 0 {
			currentQueue = append(currentQueue, id)
		}
	}

	visitedCount := 0
	for len(currentQueue) > 0 {
		var nextQueue []string
		var currentLayer []model.StepDefinition

		for _, id := range currentQueue {
			currentLayer = append(currentLayer, stepMap[id])
			visitedCount++

			for _, neighbor := range adjList[id] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					nextQueue = append(nextQueue, neighbor)
				}
			}
		}

		executionLayers = append(executionLayers, currentLayer)
		currentQueue = nextQueue
	}

	if visitedCount != len(steps) {
		return nil, errorx.NewError(errorx.ErrTypeValidation, "cycle detected in the workflow DAG", errors.New("cycle detected in the workflow DAG, infinite loop prevented"))
	}

	return executionLayers, nil
}
