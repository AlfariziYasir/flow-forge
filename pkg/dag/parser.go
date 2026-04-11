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

	// Initialize tracking maps
	for _, step := range steps {
		if _, exists := stepMap[step.ID]; exists {
			return nil, fmt.Errorf("duplicate step id found: %s", step.ID)
		}
		stepMap[step.ID] = step
		inDegree[step.ID] = 0 // Baseline in-degree
		adjList[step.ID] = []string{}
	}

	// Populate adjacency list and calculation in-degrees based on dependencies
	for _, step := range steps {
		for _, depID := range step.DependsOn {
			if _, exists := stepMap[depID]; !exists {
				return nil, fmt.Errorf("step '%s' depends on a non-existent step: '%s'", step.ID, depID)
			}
			// depID must be finished before step.ID can start
			adjList[depID] = append(adjList[depID], step.ID)
			inDegree[step.ID]++
		}
	}

	var executionLayers [][]model.StepDefinition
	var currentQueue []string

	// Find initial steps with 0 dependencies (roots)
	for id, deg := range inDegree {
		if deg == 0 {
			currentQueue = append(currentQueue, id)
		}
	}

	visitedCount := 0

	// Process queue layer by layer (Kahn's Algorithm modified for parallel groups)
	for len(currentQueue) > 0 {
		var nextQueue []string
		var currentLayer []model.StepDefinition

		for _, id := range currentQueue {
			currentLayer = append(currentLayer, stepMap[id])
			visitedCount++

			// By "resolving" this step, we reduce the in-degree of the ones waiting on it
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

	// Cycle Detection: If we haven't visited all nodes, there must be a circular dependency!
	if visitedCount != len(steps) {
		return nil, errors.New("cycle detected in the workflow DAG, infinite loop prevented")
	}

	return executionLayers, nil
}
