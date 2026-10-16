package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validateInterruptAllocationCoverage enforces ISR's mandatory allocation proof
// for operations and cleanup not yet covered by direct effect summaries. The
// ordinary @noAlloc check owns definite/unknown effects, so no duplicate is
// emitted. Generic invalid-source evidence is not an additional policy cause:
// unsupported ISR attributes remain rejected independently. Removing that marker
// here never publishes a positive ISR proof or changes AllocationFacts.
// Rules: rules/platform/interrupts.md — §§24–29;
// rules/memory/allocation.md — §§21,24(4),(6),28(4);
// rules/analysis/isr_analysis.md — §§10.1–10.2,19,22,25.2–25.3.
func (a *Analyzer) validateInterruptAllocationCoverage(program *ast.Program) {
	var unknown map[CallableID][]AllocationEvidence
	check := func(fn *ast.FunctionDeclaration, name string) {
		if fn == nil || fn.Extern || fn.Name == nil {
			return
		}
		context, exists := EffectiveGuaranteeOf(fn.Attributes, GuaranteeISR)
		if !exists {
			context, exists = EffectiveGuaranteeOf(fn.Attributes, GuaranteeInterruptSafe)
		}
		if !exists {
			return
		}
		function, exists := a.lookupFunctionByToken(name, fn.Name.Token)
		if !exists || function.ReturnType.Kind == InvalidType {
			return
		}
		id := callableID(function)
		summary := a.callGraph.ArenaSummary(id)
		if summary.MayAllocate || summary.AllocationUnknown {
			return // The owning S1108 already carries the canonical effect path.
		}
		if unknown == nil {
			unknown = a.allocationCoverageUnknownsForProgram(program)
			for node, evidence := range unknown {
				filtered := evidence[:0]
				for _, item := range evidence {
					if item.Reason != allocationInvalidSourceReason {
						filtered = append(filtered, item)
					}
				}
				unknown[node] = filtered
			}
		}
		fact := a.callGraph.allocationCallableFact(id, unknown)
		if !fact.HasUnknown {
			return // No additional cause; this does not certify ISR legality.
		}
		evidence := fact.UnknownEvidence[0]
		var chain []string
		for _, node := range fact.UnknownPath {
			chain = append(chain, a.callGraph.nodes[node].Name)
		}
		message := fmt.Sprintf("function %s cannot prove ISR @noAlloc: %s via %s", name, evidence.Reason, strings.Join(chain, " -> "))
		help := "@" + context.Source.Name.Value + " requires a complete allocation-free proof, including cleanup and compiler-generated operations; unsafe and try do not waive it. Supply verified allocation-free operations and contracts, or move this work outside ISR execution."
		before := len(a.errors)
		a.addErrorAtTokenWithMetadataAndPrevious(context.Source.Token, evidence.Source, diagnostics.NoAllocViolation, help, "%s", message)
		if len(a.errors) > before {
			e := &a.errors[len(a.errors)-1]
			e.ProofState = diagnostics.ProofUnproven
			e.RelatedLabel = "allocation coverage is incomplete here"
			cause := a.callGraph.allocationCauseForPath(fact.UnknownPath, true, &evidence)
			e.AllocationCause = &cause
		}
	}
	a.withProgramModules(program, func(statement ast.Statement) {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			if statement.Name != nil {
				check(statement, statement.Name.Value)
			}
		case *ast.ImplStatement:
			if statement.Target == nil || !a.validImplStatements[statement] {
				return
			}
			for _, member := range statement.Members {
				if fn, ok := member.(*ast.FunctionDeclaration); ok && fn.Name != nil {
					check(fn, statement.Target.Name+"."+fn.Name.Value)
				}
			}
		}
	})
}

// interruptAllocationHelp keeps repairs compatible with ISR's effective
// allocation guarantee, including when @noAlloc is redundantly explicit.
// Rules: rules/foundations/attributes.md — "ISR verification", "Interrupt-safe
// guarantee", "Attribute implications", "@noAlloc and fallible allocation";
// rules/memory/allocation.md — §28 "Diagnostics".
func interruptAllocationHelp(kind ArenaEffectKind, attribute string) string {
	prefix := "@" + attribute + " requires noAlloc on every reachable synchronous and defer path; unsafe does not waive this requirement. "
	switch kind {
	case ArenaEffectForeign:
		return prefix + "Foreign allocation behavior is unknown. Supply a documented trusted @noAlloc foreign contract, or move the call outside ISR execution; other ISR/FFI requirements still apply."
	case ArenaEffectUnknownCallee:
		return prefix + "This call's target is unknown. Use a compiler-resolved target whose allocation-free behavior can be proven, or move the call outside ISR execution."
	default:
		return prefix + "This operation acquires new storage. try only handles its failure; it still allocates. Use caller-provided or preallocated storage, or move the allocating work outside ISR execution."
	}
}
