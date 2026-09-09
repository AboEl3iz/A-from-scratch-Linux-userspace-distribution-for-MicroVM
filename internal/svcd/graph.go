package svcd

import (
	"errors"
	"fmt"
)

var ErrCyclicDependency = errors.New("cyclic service dependency detected")

// DependencyGraph manages service DAG resolution for startup ordering.
type DependencyGraph struct {
	services map[string]*ServiceSpec
}

// NewDependencyGraph creates a graph initialized with the given service specifications.
func NewDependencyGraph(specs []*ServiceSpec) *DependencyGraph {
	g := &DependencyGraph{
		services: make(map[string]*ServiceSpec),
	}
	for _, spec := range specs {
		g.services[spec.Name] = spec
	}
	return g
}

// ResolveTopologicalSort computes a valid execution order for services based on 'After' dependencies.
// Services with no dependencies appear first. Returns ErrCyclicDependency if a cycle is found.
func (g *DependencyGraph) ResolveTopologicalSort() ([]*ServiceSpec, error) {
	inDegree := make(map[string]int)
	adj := make(map[string][]string) // dependency -> dependent services

	for name := range g.services {
		inDegree[name] = 0
		adj[name] = []string{}
	}

	for name, spec := range g.services {
		for _, dep := range spec.After {
			if _, exists := g.services[dep]; exists {
				adj[dep] = append(adj[dep], name)
				inDegree[name]++
			}
		}
	}

	// Kahn's algorithm for topological sorting
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var ordered []*ServiceSpec
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if spec, exists := g.services[curr]; exists {
			ordered = append(ordered, spec)
		}

		for _, neighbor := range adj[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(ordered) != len(g.services) {
		return nil, fmt.Errorf("%w: resolved %d of %d services", ErrCyclicDependency, len(ordered), len(g.services))
	}

	return ordered, nil
}
