package sema

import (
	"math/big"

	"sec/internal/ast"
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
func (a *Analyzer) arenaElementLayout(element Type, operation string) (*ArenaElementLayout, string) {
	plan := a.targetCapabilityPlan
	requirement := LayoutRequirementOf(element)
	failure := func(state layout.LayoutCompleteness, missing ...string) string {
		return layout.DescribeLayoutFailure(plan, requirement, typeDisplayName(element), state, missing, operation)
	}
	if plan.PointerWidthBits == 0 || plan.Alignments == nil {
		return nil, failure(layout.LayoutIncomplete, "the selected target has no verified data layout")
	}
	if a.arenaLayoutResolver == nil {
		resolver, err := layout.NewResolver(plan, plan.Alignments)
		if err != nil {
			return nil, failure(layout.LayoutInvalid, err.Error())
		}
		a.arenaLayoutResolver = resolver
	}
	resolved := a.arenaLayoutResolver.Resolve(requirement)
	if resolved.Completeness != layout.LayoutComplete || resolved.SizeBytes == nil || resolved.AlignBytes == nil {
		return nil, failure(resolved.Completeness, resolved.Missing...)
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

// checkStaticallyImpossibleArenaAllocation diagnoses one New/Alloc that can
// never succeed: its checked payload count * size exceeds the plan's maximum
// byte extent, or a FromBuffer Arena's known backing is smaller than the
// payload alone (alignment padding can only add to it). A compile-time error
// is preferred over a guaranteed failing try path; the call keeps its Result
// type.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — § 83;
// rules/memory/allocation.md — §§ 23(2), 23(4); rules/memory/arena.md — §§ 16(5)–16(6).
func (a *Analyzer) checkStaticallyImpossibleArenaAllocation(call *ast.CallExpression, method string) {
	plan, ok := a.arenaOperationPlans[call]
	if !ok || plan.ElementLayout == nil || plan.Count == nil || plan.Count.Sign() <= 0 || plan.ElementLayout.SizeBytes.Sign() == 0 {
		return
	}
	payload := new(big.Int).Mul(plan.Count, plan.ElementLayout.SizeBytes)
	element := typeDisplayName(plan.ElementType)
	if a.targetCapabilityPlan.PointerWidthBits == 0 {
		// Without a plan the extent is unknown; only the buffer bound applies.
	} else if maximum := layout.MaximumExtent(a.targetCapabilityPlan); payload.Cmp(maximum) > 0 {
		a.addErrorAtTokenWithMetadata(call.Token, "",
			"allocate fewer elements; the checked size count * size of "+element+" exceeds the largest object the target can address",
			"Arena.%s can never succeed: %s elements of %s need %s bytes, more than the target's maximum object size of %s bytes",
			method, plan.Count, element, payload, maximum)
		return
	}
	if plan.BackingKind == "borrowed" && plan.Capacity.BackingBytes != nil && payload.Cmp(plan.Capacity.BackingBytes) > 0 {
		a.addErrorAtTokenWithMetadata(call.Token, "",
			"use a larger buffer or allocate fewer elements",
			"Arena.%s can never succeed: %s bytes of %s exceed the %s-byte buffer backing this Arena",
			method, payload, element, plan.Capacity.BackingBytes)
	}
}
