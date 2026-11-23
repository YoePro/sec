package sema

import "sec/internal/lexer"

// AllocationContextRequirement is the P19 callable summary of § 77: whether
// the callable needs an ambient allocation context. It is distinct from
// MayAllocate (§ 52): allocation through an explicit Arena allocates without
// requiring ambient context.
type AllocationContextRequirement string

const (
	AllocationContextNone     AllocationContextRequirement = "none"
	AllocationContextRequired AllocationContextRequirement = "required"
)

// ambientAllocationEffect reports the allocation effects that take the
// ambient context: collection/backing growth (T[] Append, core array
// materialization), string materialization and owned Arena creation. Arena New/Alloc allocate in the
// explicitly selected Arena; FromBuffer, Reset and Release allocate nothing.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 52-54, 59.
func ambientAllocationEffect(kind ArenaEffectKind) bool {
	switch kind {
	case ArenaEffectCollectionGrowth, ArenaEffectCreateOwned, ArenaEffectCreateGrowable, ArenaEffectStringMaterialization:
		return true
	}
	return false
}

// propagatesAllocationContext reports call relations that run in the
// caller's execution and so receive its context; a spawned task, thread or
// process is a new execution context that does not inherit it (§ 63).
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 56, 58, 63.
func propagatesAllocationContext(site CallSite) bool {
	return site.Execution == CallExecutionSynchronous || site.Execution == CallExecutionDeferred || site.Execution == ""
}

// AllocationContextRequirements computes the § 77 summary for every callable
// of the last analysis: a callable requires ambient context when it contains
// an ambient allocation site or synchronously calls a callable that
// requires it (a least fixed point over the call graph). Foreign callees
// never require a Sec context (§ 64).
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 52-58, 63-64, 77.
func (a *Analyzer) AllocationContextRequirements() map[CallableID]AllocationContextRequirement {
	if a == nil || a.callGraph == nil {
		return nil
	}
	if a.allocationContextRequirements != nil {
		return a.allocationContextRequirements
	}
	required := map[CallableID]bool{}
	for _, node := range a.callGraph.Nodes() {
		if node.Extern {
			continue
		}
		for _, site := range a.callGraph.arenaEffects[node.ID] {
			if (site.MayAllocate || site.UnknownAllocation) && ambientAllocationEffect(site.Kind) {
				required[node.ID] = true
				break
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, node := range a.callGraph.Nodes() {
			if required[node.ID] || node.Extern {
				continue
			}
			for _, site := range a.callGraph.Outgoing(node.ID) {
				if !propagatesAllocationContext(site) {
					continue
				}
				for _, target := range site.Targets {
					if required[target] {
						required[node.ID], changed = true, true
						break
					}
				}
				if required[node.ID] {
					break
				}
			}
		}
	}
	result := map[CallableID]AllocationContextRequirement{}
	for _, node := range a.callGraph.Nodes() {
		result[node.ID] = AllocationContextNone
		if required[node.ID] {
			result[node.ID] = AllocationContextRequired
		}
	}
	a.allocationContextRequirements = result
	return result
}

// RequiresAllocationContextAt reports the summary of the callable declared
// at token; any specialization requiring context makes it required.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — § 77.
func (a *Analyzer) RequiresAllocationContextAt(declaration lexer.Token) bool {
	requirements := a.AllocationContextRequirements()
	for _, node := range a.callGraph.NodesForDeclaration(declaration) {
		if requirements[node.ID] == AllocationContextRequired {
			return true
		}
	}
	return false
}

// CallRootKindAt reports the entry-root kind of the callable declared at
// token (program, test, task or thread entry), or "" when it is not a root.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 60, 63.
func (a *Analyzer) CallRootKindAt(declaration lexer.Token) CallRootKind {
	if a == nil || a.callGraph == nil {
		return ""
	}
	nodes := map[CallableID]bool{}
	for _, node := range a.callGraph.NodesForDeclaration(declaration) {
		nodes[node.ID] = true
	}
	for _, root := range a.callGraph.Roots() {
		if nodes[root.Node] {
			return root.Kind
		}
	}
	return ""
}
