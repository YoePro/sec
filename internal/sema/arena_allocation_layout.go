package sema

import (
	"math/big"
	"strings"

	"sec/internal/layout"
)

// ArenaElementLayout is the resolved layout of an Arena New/Alloc element
// under the active CompilationPlan, taken from the one canonical layout
// resolver (MD-141/MD-142): size and alignment in bytes and the plan key.
// Rules: rules/memory/arena.md — §§ 16–17, 19(1); rules/memory/layout.md — § 47.5.
type ArenaElementLayout struct {
	TypeIdentity string
	PlanKey      string
	SizeBytes    *big.Int
	AlignBytes   *big.Int
}

// arenaElementLayout resolves T through the canonical resolver of the
// analyzer's plan. A plan without verified scalar alignments, or an
// incomplete layout, yields the named missing fact instead of a guess.
// Rules: rules/memory/arena.md — §§ 17(1)–17(2), 19(1); rules/memory/layout.md — §§ 47.1–47.5.
func (a *Analyzer) arenaElementLayout(element Type) (*ArenaElementLayout, string) {
	plan := a.targetCapabilityPlan
	if plan.PointerWidthBits == 0 || plan.Alignments == nil {
		return nil, "CompilationPlan layout of T (the selected target has no verified data layout)"
	}
	if a.arenaLayoutResolver == nil {
		resolver, err := layout.NewResolver(plan, plan.Alignments)
		if err != nil {
			return nil, "CompilationPlan layout of T (" + err.Error() + ")"
		}
		a.arenaLayoutResolver = resolver
	}
	resolved := a.arenaLayoutResolver.Resolve(LayoutRequirementOf(element))
	if resolved.Completeness != layout.LayoutComplete || resolved.SizeBytes == nil || resolved.AlignBytes == nil {
		return nil, "CompilationPlan layout of T (" + strings.Join(resolved.Missing, "; ") + ")"
	}
	return &ArenaElementLayout{
		TypeIdentity: resolved.TypeIdentity,
		PlanKey:      a.arenaLayoutResolver.Key().String(),
		SizeBytes:    new(big.Int).Set(resolved.SizeBytes),
		AlignBytes:   new(big.Int).Set(resolved.AlignBytes),
	}, ""
}

// IsLowerableArenaAllocation reports an Arena New/Alloc allocation site whose
// plan is complete on a borrowed fixed FromBuffer Arena: Semantic IR
// represents it as a checked arena.new/arena.alloc carrying the selected
// domain, epoch, layout, default and capacity plans, so the allocation
// readiness gate may admit exactly that operation (P19-20).
// Rules: rules/memory/allocation.md — §§ 25(1)–(5), 26(1)–(3);
// rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 113–114.
func (a *Analyzer) IsLowerableArenaAllocation(fact AllocationContextFact) bool {
	if a == nil || fact.Kind != ArenaEffectAllocate {
		return false
	}
	for call, plan := range a.arenaOperationPlans {
		if call == nil || (plan.Operation != ArenaOperationNew && plan.Operation != ArenaOperationAlloc) {
			continue
		}
		if plan.Source.File == fact.Source.File && plan.Source.Line == fact.Source.Line && plan.Source.Column == fact.Source.Column {
			return len(plan.Unresolved) == 0 && plan.BackingKind == "borrowed" && plan.ElementLayout != nil && plan.Reference != nil
		}
	}
	return false
}
