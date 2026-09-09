package svcd

import (
	"testing"
)

func TestGraphSort_Linear(t *testing.T) {
	specs := []*ServiceSpec{
		{Name: "web", After: []string{"api"}},
		{Name: "api", After: []string{"db"}},
		{Name: "db", After: []string{}},
	}

	graph := NewDependencyGraph(specs)
	ordered, err := graph.ResolveTopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error during topological sort: %v", err)
	}

	if len(ordered) != 3 {
		t.Fatalf("expected 3 services, got %d", len(ordered))
	}

	// db must be first, api second, web third
	if ordered[0].Name != "db" {
		t.Errorf("expected db first, got %s", ordered[0].Name)
	}
	if ordered[1].Name != "api" {
		t.Errorf("expected api second, got %s", ordered[1].Name)
	}
	if ordered[2].Name != "web" {
		t.Errorf("expected web third, got %s", ordered[2].Name)
	}
}

func TestGraphSort_Cycle(t *testing.T) {
	specs := []*ServiceSpec{
		{Name: "serviceA", After: []string{"serviceB"}},
		{Name: "serviceB", After: []string{"serviceA"}},
	}

	graph := NewDependencyGraph(specs)
	_, err := graph.ResolveTopologicalSort()
	if err == nil {
		t.Fatal("expected cyclic dependency error, got nil")
	}
}

func TestGraphSort_Independent(t *testing.T) {
	specs := []*ServiceSpec{
		{Name: "serviceA"},
		{Name: "serviceB"},
		{Name: "serviceC"},
	}

	graph := NewDependencyGraph(specs)
	ordered, err := graph.ResolveTopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ordered) != 3 {
		t.Fatalf("expected 3 services, got %d", len(ordered))
	}
}
