package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
)

// validateNoAllocTargetRoots enforces a positively resolved disabled-allocation
// policy over complete reachable root graphs, including spawned execution and
// deferred work. Unknown operation/body/callee coverage cannot prove freedom.
// Unreachable declarations create no root obligations; invalid source retains
// its prior owning errors instead of acquiring speculative policy cascades.
// Rules: rules/memory/allocation.md — §§6(8),22,24(2),(6),28(4);
// rules/analysis/call_graph.md — "Roots", "Execution relations".
func (a *Analyzer) validateNoAllocTargetRoots(program *ast.Program) {
	capabilities := a.AllocationCapabilities()
	if !capabilities.ProfileKnown || capabilities.DynamicAllocation.Activation != layout.AllocationDisabled || len(a.errors) != 0 {
		return
	}
	unknown := a.allocationCoverageUnknownsForProgram(program)
	// Freeze all root obligations before emitting any diagnostic: one root's
	// policy failure must not turn another root's completed coverage unknown.
	for _, id := range a.callGraph.rootOrder {
		root := a.callGraph.roots[id]
		paths := a.allocationPolicyPaths(root.Node)
		var source lexer.Token
		var path []CallableID
		reason := ""
		unproven := false
		for _, candidate := range paths {
			for _, site := range a.callGraph.arenaEffects[candidate[len(candidate)-1]] {
				if site.MayAllocate {
					source, path, reason = site.Source, candidate, "reachable operation may allocate"
					break
				}
			}
			if path != nil {
				break
			}
		}
		if path == nil {
			for _, candidate := range paths {
				if evidence := unknown[candidate[len(candidate)-1]]; len(evidence) > 0 {
					source, path, reason, unproven = evidence[0].Source, candidate, evidence[0].Reason, true
					break
				}
			}
		}
		if path == nil {
			continue
		}
		var names []string
		for _, node := range path {
			names = append(names, a.callGraph.nodes[node].Name)
		}
		message := fmt.Sprintf("noalloc profile requires an allocation-free reachable graph from %s: %s via %s", a.callGraph.nodes[root.Node].Name, reason, strings.Join(names, " -> "))
		help := "Use allocation-free operations and callees with verified allocation-free contracts, or select a profile that permits allocation. try handles failures but does not remove allocation."
		a.addErrorAtTokenWithMetadataAndPrevious(root.Source, source, diagnostics.NoAllocViolation, help, "%s", message)
		if unproven {
			a.errors[len(a.errors)-1].ProofState = diagnostics.ProofUnproven
		} else {
			a.errors[len(a.errors)-1].ProofState = diagnostics.ProofInvalid
		}
		a.errors[len(a.errors)-1].RelatedLabel = "allocation policy cause"
		cause := a.allocationTargetCause(path, source, reason, unproven)
		a.errors[len(a.errors)-1].AllocationCause = &cause
	}
}

// allocationTargetCause binds the policy witness to canonical root/call-site
// identities, including worker execution, so CLI and LSP share navigation.
// Rules: rules/memory/allocation.md — §§28(4),29(1),(4);
// rules/analysis/call_graph.md — "Execution relations".
func (a *Analyzer) allocationTargetCause(path []CallableID, source lexer.Token, reason string, unknown bool) AllocationCausePath {
	result := AllocationCausePath{Unknown: unknown, Path: append([]CallableID(nil), path...)}
	root := a.callGraph.nodes[path[0]]
	result.Steps = append(result.Steps, AllocationCauseStep{Kind: "root", Source: root.Declaration, Caller: root.ID, Message: "noalloc policy root: " + root.Name})
	for index := 1; index < len(path); index++ {
		found := false
		for _, site := range a.callGraph.Outgoing(path[index-1]) {
			if !executionContributesReachability(site.Execution) {
				continue
			}
			for _, target := range site.Targets {
				if target != path[index] {
					continue
				}
				result.Steps = append(result.Steps, AllocationCauseStep{Kind: "call", Source: site.Source, Caller: site.Caller, Callee: target, Site: site.ID, Message: "allocation policy reaches callee through " + string(site.Execution)})
				found = true
				break
			}
			if found {
				break
			}
		}
		if !found {
			result.Incomplete = true
		}
	}
	result.Steps = append(result.Steps, AllocationCauseStep{Kind: "operation", Source: source, Caller: path[len(path)-1], Message: reason})
	for _, step := range result.Steps {
		if step.Source.Line <= 0 || step.Source.Column <= 0 {
			result.Incomplete = true
		}
	}
	return result
}

// allocationPolicyPaths returns deterministic shortest root-to-callable paths
// across all graph reachability relations, rather than same-stack summaries.
// Each canonical node is visited once, so recursive components terminate.
// Rules: rules/memory/allocation.md — §6(8),24(4);
// rules/analysis/call_graph.md — "Execution relations", "Roots".
func (a *Analyzer) allocationPolicyPaths(root CallableID) [][]CallableID {
	paths := [][]CallableID{{root}}
	seen := map[CallableID]bool{root: true}
	for index := 0; index < len(paths); index++ {
		path := paths[index]
		for _, target := range a.callGraph.reachabilityTargets(path[len(path)-1]) {
			if seen[target] {
				continue
			}
			seen[target] = true
			paths = append(paths, append(append([]CallableID(nil), path...), target))
		}
	}
	return paths
}
