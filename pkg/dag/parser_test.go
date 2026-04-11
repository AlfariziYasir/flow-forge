package dag_test

import (
	"testing"
	"flowforge/internal/model"
	"flowforge/pkg/dag"
)

func TestBuildExecutionPlan_Success(t *testing.T) {
	// Setup a simple DAG:
	// A -> B
	// A -> C
	// B, C -> D
	steps := []model.StepDefinition{
		{ID: "A", Action: "http_get", DependsOn: []string{}},
		{ID: "B", Action: "script", DependsOn: []string{"A"}},
		{ID: "C", Action: "wait", DependsOn: []string{"A"}},
		{ID: "D", Action: "http_post", DependsOn: []string{"B", "C"}},
	}

	layers, err := dag.BuildExecutionPlan(steps)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(layers) != 3 {
		t.Fatalf("expected 3 execution layers, got %d", len(layers))
	}

	// Layer 0 should only contain A
	if len(layers[0]) != 1 || layers[0][0].ID != "A" {
		t.Errorf("expected Layer 0 to be [A], got %v", getIDs(layers[0]))
	}

	// Layer 1 should contain B and C (parallel execution)
	if len(layers[1]) != 2 {
		t.Errorf("expected Layer 1 to have 2 steps, got %v", getIDs(layers[1]))
	}

	// Layer 2 should contain D
	if len(layers[2]) != 1 || layers[2][0].ID != "D" {
		t.Errorf("expected Layer 2 to be [D], got %v", getIDs(layers[2]))
	}
}

func TestBuildExecutionPlan_CycleDetection(t *testing.T) {
	// Setup a DAG with a cycle:
	// A -> B -> C -> A
	steps := []model.StepDefinition{
		{ID: "A", Action: "http", DependsOn: []string{"C"}},
		{ID: "B", Action: "http", DependsOn: []string{"A"}},
		{ID: "C", Action: "http", DependsOn: []string{"B"}},
	}

	_, err := dag.BuildExecutionPlan(steps)
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}
}

func TestBuildExecutionPlan_MissingDependency(t *testing.T) {
	steps := []model.StepDefinition{
		{ID: "A", Action: "http", DependsOn: []string{"Z"}}, // Z does not exist
	}

	_, err := dag.BuildExecutionPlan(steps)
	if err == nil {
		t.Fatal("expected missing dependency error, got nil")
	}
}

// Helper to extract IDs for printing/debugging
func getIDs(steps []model.StepDefinition) []string {
	ids := make([]string, len(steps))
	for i, step := range steps {
		ids[i] = step.ID
	}
	return ids
}
