package modules

import (
	"fmt"
	"sort"
)

// ModuleInstanceSelection records the logical module and exact source files
// selected for one CompilationPlan. Per rules/projects/modules.md, source
// selection belongs to the ModuleInstance and never mutates ModuleIdentity.
type ModuleInstanceSelection struct {
	Instance        ModuleInstance
	SelectedSources []string
}

// ModuleDependencyFacts is an immutable-by-convention snapshot for downstream
// consumers such as rules/compiler/initialization.md. Imports are semantic
// dependency facts only: their order is deterministic storage order and is not
// a runtime initialization or deinitialization order.
type ModuleDependencyFacts struct {
	CompilationPlanID string
	Modules           []ModuleInstanceSelection
	Imports           []ImportEdge
}

// ModuleInstanceGraph owns the selected module graph for one concrete
// CompilationPlan as required by rules/projects/modules.md. Different Variants
// use different graphs even when they share logical ModuleIdentity values.
type ModuleInstanceGraph struct {
	compilationPlanID string
	logical           *ModuleGraph
	selections        map[ModuleIdentity][]string
}

// NewModuleInstanceGraph creates an empty graph for exactly one concrete
// CompilationPlan. An empty plan identity cannot provide plan-scoped module
// identity and is rejected.
func NewModuleInstanceGraph(compilationPlanID string) (*ModuleInstanceGraph, error) {
	if compilationPlanID == "" {
		return nil, fmt.Errorf("module-instance graph requires a compilation-plan identity")
	}
	return &ModuleInstanceGraph{
		compilationPlanID: compilationPlanID,
		logical:           NewModuleGraph(),
		selections:        map[ModuleIdentity][]string{},
	}, nil
}

// AddModule selects one logical module and its exact active source files for
// this graph's CompilationPlan. Repeating an identical selection is harmless;
// changing it in place is rejected so plan facts remain stable.
func (graph *ModuleInstanceGraph) AddModule(identity ModuleIdentity, selectedSources []string) error {
	if graph == nil {
		return fmt.Errorf("cannot add a module to a nil module-instance graph")
	}
	if err := validateModuleIdentity(identity); err != nil {
		return err
	}
	sources, err := canonicalSelectedSources(selectedSources)
	if err != nil {
		return err
	}
	if existing, ok := graph.selections[identity]; ok {
		if !equalStrings(existing, sources) {
			return fmt.Errorf("module %s already has a different source selection in plan %s", identity, graph.compilationPlanID)
		}
		return nil
	}
	graph.selections[identity] = sources
	graph.logical.AddModule(identity)
	return nil
}

// AddImport adds a resolved import between two modules selected into the same
// CompilationPlan. Cross-plan or unselected destinations are rejected before
// the canonical logical edge is recorded.
func (graph *ModuleInstanceGraph) AddImport(edge ImportEdge) error {
	if graph == nil {
		return fmt.Errorf("cannot add an import to a nil module-instance graph")
	}
	if _, ok := graph.selections[edge.From]; !ok {
		return fmt.Errorf("importer %s is not selected in plan %s", edge.From, graph.compilationPlanID)
	}
	if _, ok := graph.selections[edge.To]; !ok {
		return fmt.Errorf("import destination %s is not selected in plan %s", edge.To, graph.compilationPlanID)
	}
	return graph.logical.AddImport(edge)
}

// Instance returns the plan-qualified identity for a selected logical module.
func (graph *ModuleInstanceGraph) Instance(identity ModuleIdentity) (ModuleInstance, bool) {
	if graph == nil {
		return ModuleInstance{}, false
	}
	if _, ok := graph.selections[identity]; !ok {
		return ModuleInstance{}, false
	}
	return ModuleInstance{Identity: identity, CompilationPlanID: graph.compilationPlanID}, true
}

// TopologicalOrder exposes dependency-first semantic processing order for this
// plan. As specified by rules/projects/modules.md, callers must not reuse this
// order as runtime initialization or deinitialization order.
func (graph *ModuleInstanceGraph) TopologicalOrder() ([]ModuleInstance, []Cycle) {
	if graph == nil {
		return nil, nil
	}
	identities, cycles := graph.logical.TopologicalOrder()
	if len(cycles) != 0 {
		return nil, cycles
	}
	instances := make([]ModuleInstance, 0, len(identities))
	for _, identity := range identities {
		instances = append(instances, ModuleInstance{Identity: identity, CompilationPlanID: graph.compilationPlanID})
	}
	return instances, nil
}

// DependencyFacts returns defensive copies of selected-module and import facts
// for initialization and other downstream analyses. The Modules slice is
// canonical identity order, deliberately not dependency-first order.
func (graph *ModuleInstanceGraph) DependencyFacts() ModuleDependencyFacts {
	if graph == nil {
		return ModuleDependencyFacts{}
	}
	identities := make([]ModuleIdentity, 0, len(graph.selections))
	for identity := range graph.selections {
		identities = append(identities, identity)
	}
	sortIdentities(identities)
	modules := make([]ModuleInstanceSelection, 0, len(identities))
	for _, identity := range identities {
		modules = append(modules, ModuleInstanceSelection{
			Instance:        ModuleInstance{Identity: identity, CompilationPlanID: graph.compilationPlanID},
			SelectedSources: append([]string(nil), graph.selections[identity]...),
		})
	}
	return ModuleDependencyFacts{
		CompilationPlanID: graph.compilationPlanID,
		Modules:           modules,
		Imports:           graph.logical.ImportEdges(),
	}
}

// validateModuleIdentity checks a previously constructed identity at API
// boundaries without redefining its language semantics.
func validateModuleIdentity(identity ModuleIdentity) error {
	if identity.ImportRootIdentity == "" {
		return fmt.Errorf("module import-root identity is empty")
	}
	if err := ValidateImportPath(identity.CanonicalImportPath); err != nil {
		return fmt.Errorf("invalid module identity %s: %w", identity, err)
	}
	return nil
}

// canonicalSelectedSources copies, sorts, and deduplicates an opaque set of
// source-file identities without applying host filesystem normalization.
func canonicalSelectedSources(sources []string) ([]string, error) {
	result := append([]string(nil), sources...)
	for _, source := range result {
		if source == "" {
			return nil, fmt.Errorf("selected module source identity is empty")
		}
	}
	sort.Strings(result)
	write := 0
	for _, source := range result {
		if write != 0 && result[write-1] == source {
			continue
		}
		result[write] = source
		write++
	}
	return result[:write], nil
}

// equalStrings compares two canonical string slices without allocating.
func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
