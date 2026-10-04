package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// validateNoAllocGuarantees enforces the transitive @noAlloc guarantee on
// every Sec function and method that writes it: no reachable synchronous path
// may perform allocation or reach a call whose allocation behavior is unknown.
// On an extern declaration @noAlloc is a trusted foreign contract instead.
//
// Rules:
//   - rules/foundations/attributes.md — "@noAlloc", "Allocation definition", "@noAlloc verification", "@noAlloc and fallible allocation"
//   - rules/memory/allocation.md — § 24 "Allocation effects"
func (a *Analyzer) validateNoAllocGuarantees(program *ast.Program) {
	a.withProgramModules(program, func(statement ast.Statement) {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			a.validateFunctionNoAllocGuarantee(statement, statement.Name.Value)
		case *ast.ImplStatement:
			if statement.Target == nil || !a.validImplStatements[statement] {
				return
			}
			for _, member := range statement.Members {
				fn, ok := member.(*ast.FunctionDeclaration)
				if ok && fn != nil {
					a.validateFunctionNoAllocGuarantee(fn, statement.Target.Name+"."+fn.Name.Value)
				}
			}
		}
	})
}

func (a *Analyzer) validateFunctionNoAllocGuarantee(fn *ast.FunctionDeclaration, name string) {
	if fn.Extern || fn.Name == nil {
		return
	}
	var attribute *ast.Attribute
	for _, candidate := range fn.Attributes {
		if candidate != nil && candidate.Name != nil && candidate.Name.Value == "noAlloc" {
			attribute = candidate
			break
		}
	}
	if attribute == nil {
		return
	}
	function, ok := a.lookupFunctionByToken(name, fn.Name.Token)
	if !ok || function.ReturnType.Kind == InvalidType {
		return
	}
	summary := a.callGraph.ArenaSummary(callableID(function))
	path, unknown := summary.AllocationPath, false
	if !summary.MayAllocate {
		if !summary.AllocationUnknown {
			return
		}
		path, unknown = summary.UnknownAllocationPath, true
	}
	if len(path) == 0 {
		return
	}
	chain := make([]string, 0, len(path))
	for _, id := range path {
		if node, exists := a.callGraph.Node(id); exists {
			chain = append(chain, node.Name)
		}
	}
	site, found := introducingAllocationSite(a.callGraph.ArenaSummary(path[len(path)-1]).DirectEffects, unknown)
	kind := "allocation"
	if unknown {
		kind = "unknown allocation behavior"
	}
	message := fmt.Sprintf("function %s does not satisfy @noAlloc: reachable %s via %s", name, kind, strings.Join(chain, " -> "))
	if found {
		message += fmt.Sprintf("; introduced at %s", formatLocation(site.Source.File, site.Source.Line, site.Source.Column))
	}
	help := noAllocEffectHelp(site.Kind)
	if len(chain) > 1 {
		help = "It comes from " + chain[len(chain)-1] + ", which " + name + " calls. " + help
	}
	if found {
		a.addErrorAtTokenWithMetadataAndPrevious(attribute.Token, site.Source, diagnostics.NoAllocViolation, help, "%s", message)
		return
	}
	a.addErrorAtTokenWithMetadata(attribute.Token, diagnostics.NoAllocViolation, help, "%s", message)
}

// introducingAllocationSite picks the first direct site of the requested
// class: a definite allocation, or an unknown allocation behavior.
func introducingAllocationSite(effects []ArenaEffectSite, unknown bool) (ArenaEffectSite, bool) {
	for _, effect := range effects {
		if (!unknown && effect.MayAllocate || unknown && effect.UnknownAllocation) && effect.Source.Line > 0 {
			return effect, true
		}
	}
	return ArenaEffectSite{Source: lexer.Token{}}, false
}

// noAllocEffectHelp explains, in programmer terms, why a site breaks the
// @noAlloc guarantee and how to avoid it.
//
// Rules:
//   - rules/foundations/attributes.md — "@noAlloc and fallible allocation"
//   - rules/memory/allocation.md — § 28 "Diagnostics"
func noAllocEffectHelp(kind ArenaEffectKind) string {
	switch kind {
	case ArenaEffectAllocate:
		return "This operation acquires new storage (an Arena allocation, string concatenation or interpolation, or another allocating operation). try only handles its failure; it still allocates. Use caller-provided or preallocated storage, or remove @noAlloc."
	case ArenaEffectUnknownCallee:
		return "This call's target is not known here (a function value, an interface method, or a generic constraint method), so whether it allocates cannot be proven. Call a known function or concrete method directly, or remove @noAlloc."
	case ArenaEffectForeign:
		return "A call to an extern function has unknown foreign behavior and may allocate. When the foreign documentation guarantees it does not allocate, mark the extern declaration @noAlloc as a trusted foreign contract."
	}
	return "Remove the allocating operation from every path reachable from this function, or remove @noAlloc."
}
