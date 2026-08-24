package modules

import (
	"fmt"
	"sort"
	"strings"
)

// ImportEdge records one resolved module dependency and its navigable source
// location as required by rules/projects/modules.md.
type ImportEdge struct {
	From       ModuleIdentity
	To         ModuleIdentity
	SourceFile string
	Line       int
	Column     int
}

// Cycle is one deterministic representative module-import cycle together with
// the exact import edges that form its path.
type Cycle struct {
	Modules []ModuleIdentity
	Edges   []ImportEdge
}

// ModuleCycleDiagnostic is the compiler-owned, navigable cycle report required
// by rules/projects/modules.md. Primary is always the edge leaving the canonical
// first module; Related retains the remaining import sites in traversal order.
type ModuleCycleDiagnostic struct {
	Message string
	Primary ImportEdge
	Related []ImportEdge
}

// String returns the representative cycle as a readable closed module path.
func (cycle Cycle) String() string {
	parts := make([]string, 0, len(cycle.Modules))
	for _, module := range cycle.Modules {
		parts = append(parts, module.String())
	}
	return strings.Join(parts, " -> ")
}

// Diagnostic converts the canonical representative from
// rules/projects/modules.md into a stable primary location plus ordered related
// import locations suitable for compiler and LSP navigation.
func (cycle Cycle) Diagnostic() ModuleCycleDiagnostic {
	diagnostic := ModuleCycleDiagnostic{Message: "module import cycle: " + cycle.String()}
	if len(cycle.Edges) == 0 {
		return diagnostic
	}
	diagnostic.Primary = cycle.Edges[0]
	diagnostic.Related = append([]ImportEdge(nil), cycle.Edges[1:]...)
	return diagnostic
}

// ModuleGraph stores canonical module identities and their resolved dependency
// edges independently of parser, Sema, filesystem, and traversal order.
type ModuleGraph struct {
	nodes map[ModuleIdentity]struct{}
	edges map[ModuleIdentity]map[ModuleIdentity]ImportEdge
}

// NewModuleGraph creates an empty canonical dependency graph.
func NewModuleGraph() *ModuleGraph {
	return &ModuleGraph{
		nodes: map[ModuleIdentity]struct{}{},
		edges: map[ModuleIdentity]map[ModuleIdentity]ImportEdge{},
	}
}

// AddModule ensures that an identity participates in the graph even when it
// has no imports.
func (graph *ModuleGraph) AddModule(identity ModuleIdentity) {
	graph.nodes[identity] = struct{}{}
}

// AddImport applies the self-import and duplicate-edge prohibitions from
// rules/projects/modules.md while retaining the edge's source location.
func (graph *ModuleGraph) AddImport(edge ImportEdge) error {
	if edge.From == edge.To {
		return fmt.Errorf("module %s cannot import itself", edge.From)
	}
	graph.AddModule(edge.From)
	graph.AddModule(edge.To)
	if graph.edges[edge.From] == nil {
		graph.edges[edge.From] = map[ModuleIdentity]ImportEdge{}
	}
	if _, exists := graph.edges[edge.From][edge.To]; exists {
		return fmt.Errorf("module %s imports %s more than once", edge.From, edge.To)
	}
	graph.edges[edge.From][edge.To] = edge
	return nil
}

// ImportEdges returns defensive copies of all resolved imports in canonical
// importer/destination/location order. The order is deterministic storage and
// diagnostic order from rules/projects/modules.md, not initialization order.
func (graph *ModuleGraph) ImportEdges() []ImportEdge {
	if graph == nil {
		return nil
	}
	edges := make([]ImportEdge, 0)
	for _, from := range graph.sortedNodes() {
		for _, to := range graph.outgoing(from) {
			edges = append(edges, graph.edges[from][to])
		}
	}
	return edges
}

// Cycles returns one deterministic representative cycle per cyclic strongly
// connected component. Both SCC traversal and representative selection use
// canonical ModuleIdentity ordering.
func (graph *ModuleGraph) Cycles() []Cycle {
	components := graph.stronglyConnectedComponents()
	cycles := make([]Cycle, 0, len(components))
	for _, component := range components {
		if len(component) < 2 {
			continue
		}
		cycles = append(cycles, graph.representativeCycle(component))
	}
	sort.Slice(cycles, func(i, j int) bool { return cycles[i].String() < cycles[j].String() })
	return cycles
}

// TopologicalOrder returns dependencies before their importers. Cyclic graphs
// return their deterministic cycles instead of a partial order.
func (graph *ModuleGraph) TopologicalOrder() ([]ModuleIdentity, []Cycle) {
	if cycles := graph.Cycles(); len(cycles) != 0 {
		return nil, cycles
	}
	visited := map[ModuleIdentity]bool{}
	order := make([]ModuleIdentity, 0, len(graph.nodes))
	var visit func(ModuleIdentity)
	visit = func(module ModuleIdentity) {
		if visited[module] {
			return
		}
		visited[module] = true
		for _, dependency := range graph.outgoing(module) {
			visit(dependency)
		}
		order = append(order, module)
	}
	for _, module := range graph.sortedNodes() {
		visit(module)
	}
	return order, nil
}

func (graph *ModuleGraph) stronglyConnectedComponents() [][]ModuleIdentity {
	index := 0
	indices := map[ModuleIdentity]int{}
	lowlink := map[ModuleIdentity]int{}
	onStack := map[ModuleIdentity]bool{}
	stack := []ModuleIdentity{}
	components := [][]ModuleIdentity{}
	var connect func(ModuleIdentity)
	connect = func(module ModuleIdentity) {
		indices[module] = index
		lowlink[module] = index
		index++
		stack = append(stack, module)
		onStack[module] = true

		for _, dependency := range graph.outgoing(module) {
			dependencyIndex, seen := indices[dependency]
			if !seen {
				connect(dependency)
				if lowlink[dependency] < lowlink[module] {
					lowlink[module] = lowlink[dependency]
				}
			} else if onStack[dependency] && dependencyIndex < lowlink[module] {
				lowlink[module] = dependencyIndex
			}
		}

		if lowlink[module] != indices[module] {
			return
		}
		component := []ModuleIdentity{}
		for {
			last := len(stack) - 1
			member := stack[last]
			stack = stack[:last]
			onStack[member] = false
			component = append(component, member)
			if member == module {
				break
			}
		}
		sortIdentities(component)
		components = append(components, component)
	}

	for _, module := range graph.sortedNodes() {
		if _, seen := indices[module]; !seen {
			connect(module)
		}
	}
	return components
}

func (graph *ModuleGraph) representativeCycle(component []ModuleIdentity) Cycle {
	allowed := map[ModuleIdentity]bool{}
	for _, module := range component {
		allowed[module] = true
	}
	start := component[0]
	path := []ModuleIdentity{start}
	edges := []ImportEdge{}
	inPath := map[ModuleIdentity]bool{start: true}
	var found bool
	var search func(ModuleIdentity)
	search = func(current ModuleIdentity) {
		if found {
			return
		}
		for _, next := range graph.outgoing(current) {
			if !allowed[next] {
				continue
			}
			edge := graph.edges[current][next]
			if next == start {
				path = append(path, start)
				edges = append(edges, edge)
				found = true
				return
			}
			if inPath[next] {
				continue
			}
			inPath[next] = true
			path = append(path, next)
			edges = append(edges, edge)
			search(next)
			if found {
				return
			}
			edges = edges[:len(edges)-1]
			path = path[:len(path)-1]
			delete(inPath, next)
		}
	}
	search(start)
	return Cycle{Modules: path, Edges: edges}
}

func (graph *ModuleGraph) outgoing(module ModuleIdentity) []ModuleIdentity {
	out := make([]ModuleIdentity, 0, len(graph.edges[module]))
	for dependency := range graph.edges[module] {
		out = append(out, dependency)
	}
	sortIdentities(out)
	return out
}

func (graph *ModuleGraph) sortedNodes() []ModuleIdentity {
	nodes := make([]ModuleIdentity, 0, len(graph.nodes))
	for module := range graph.nodes {
		nodes = append(nodes, module)
	}
	sortIdentities(nodes)
	return nodes
}

func sortIdentities(identities []ModuleIdentity) {
	sort.Slice(identities, func(i, j int) bool { return identities[i].String() < identities[j].String() })
}
