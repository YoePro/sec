package sema

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/layout"
)

// MD-136 (arena.md § 131; default_values.md § 65B): Arena is Defaultable.
// `Arena {}` and an omitted Arena initialization are the builtin default
// creation: a fresh ArenaDomain with zero capacity, no owned or borrowed
// backing, infallible and allocation-free in every profile, with a growth
// policy frozen from the plan. No allocation provider is verified for any
// target yet (MD-151): under the interim hosted mapping (positively
// supported, enabled compiler-managed allocation) the default freezes a
// growable non-relocating policy whose provider is explicitly the interim
// hosted mapping pending MD-151; under every other policy (noalloc,
// freestanding, unknown) it is a non-growing empty Arena.

// ArenaDefaultConstructor names the builtin default creation in plans and
// creation facts; it is not a source-level static member.
const ArenaDefaultConstructor = "Default"

// ArenaDefault is the DefaultKind of the builtin Arena default.
const ArenaDefault DefaultKind = "arena"

// isBuiltinArenaType reports the semantic builtin Arena, not a user type
// that happens to share the spelling.
// Rules: rules/memory/arena.md — §§ 5, 131(2).
func isBuiltinArenaType(typ Type) bool {
	return typ.Kind == StructType && typ.Name == "Arena" && typ.Module == "" && !typ.Declared
}

// inferArenaDefaultLiteral types `Arena {}`: no fields, a fresh domain per
// evaluation, and the published default-creation plan.
// Rules: rules/memory/arena.md — §§ 131(2)–(4), 131(14).
func (a *Analyzer) inferArenaDefaultLiteral(expr *ast.StructLiteral) (Type, expressionValue) {
	if len(expr.Fields) != 0 {
		a.addErrorAtTokenWithMetadata(expr.Token, "",
			"write `Arena {}` for the empty default Arena, or use Arena.FromBuffer, Arena.WithCapacity or Arena.Growable for backing",
			"Arena {} takes no fields: it is the builtin default Arena, not a struct with accessible fields")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	arena := a.types["Arena"]
	arena.ArenaDomainID = a.newArenaDomainID(expr.Token, "creation")
	if !a.summaryPass {
		if a.arenaCreations == nil {
			a.arenaCreations = map[string]ArenaCreationFact{}
		}
		a.arenaCreations[arena.ArenaDomainID] = ArenaCreationFact{Domain: arena.ArenaDomainID, Constructor: ArenaDefaultConstructor, Source: expr.Token}
		a.recordArenaDefaultPlan(expr, arena)
	}
	return arena, expressionValue{Display: expr.String()}
}

// recordArenaDefaultPlan publishes the resolved default-creation plan: the
// fresh domain at its initial epoch and state version 1, no backing, zero
// capacity, the frozen non-growing policy and an explicitly absent provider.
// Rules: rules/memory/arena.md — §§ 131(4)–(5), 131(8)–(12), 131(14).
func (a *Analyzer) recordArenaDefaultPlan(expr *ast.StructLiteral, arena Type) {
	if !a.callGraphPathReachable {
		return
	}
	plan := ResolvedArenaOperationPlan{
		Operation: ArenaOperationCreate, Constructor: ArenaDefaultConstructor, Domain: arena.ArenaDomainID, Source: expr.Token,
		InvalidationDomain: a.placeIdentities.invalidationDomain("arena:" + arena.ArenaDomainID),
		EpochAfter:         a.placeIdentities.epochDependency(fmt.Sprintf("arena:%s@%d", arena.ArenaDomainID, 0)),
		ResultType:         semanticSnapshotType(arena), SuccessType: semanticSnapshotType(arena),
		CapacityUnit: "bytes", BackingKind: "none", GrowthKind: "fixed",
		BackingRelation: BackingRelationOwned, ReclamationAuthority: ReclamationNone,
		CapacityPlan: "empty-default", ProviderPlan: "absent", StateBefore: 0, StateAfter: 1,
		Capacity: ArenaCapacityFacts{RequestedBytes: big.NewInt(0), BackingBytes: big.NewInt(0)},
	}
	if a.defaultArenaGrows() {
		plan.GrowthKind, plan.ProviderPlan = "growable-nonrelocating", defaultArenaInterimProvider
	}
	if a.arenaDefaultPlans == nil {
		a.arenaDefaultPlans = map[*ast.StructLiteral]ResolvedArenaOperationPlan{}
	}
	a.arenaDefaultPlans[expr] = plan
}

// defaultArenaInterimProvider names the interim hosted mapping that stands
// in for a verified provider until the MD-151 catalogs exist.
const defaultArenaInterimProvider = "interim-hosted-profile (pending MD-151)"

// defaultArenaGrows reports whether the frozen plan lets a default Arena
// grow later: only a positively supported, enabled compiler-managed
// allocation capability does; unknown availability never does (§ 131(11)).
// Rules: rules/memory/arena.md — §§ 131(8)–(11); rules/memory/allocation.md — § 22(8).
func (a *Analyzer) defaultArenaGrows() bool {
	capabilities := a.AllocationCapabilities()
	return capabilities.DynamicAllocation.Availability == layout.AllocationSupported &&
		capabilities.DynamicAllocation.Activation == layout.AllocationEnabled &&
		capabilities.ContextProvider == layout.AllocationCompilerContext
}

// isDefaultArenaDomain reports a domain created by the builtin default.
// Rules: rules/memory/arena.md — § 131(10).
func (a *Analyzer) isDefaultArenaDomain(domain string) bool {
	creation, ok := a.arenaCreations[domain]
	return ok && creation.Constructor == ArenaDefaultConstructor
}

// checkDefaultArenaPositiveStorage rejects New/Alloc that need positive
// backing on a non-growing default Arena: the frozen plan has no positively
// supported compatible allocation capability, so it never grows
// (§ 131(10)-(11)). A provably zero count needs no storage and stays legal.
// Rules: rules/memory/arena.md — §§ 22, 131(10)–(11), 131(15).
func (a *Analyzer) checkDefaultArenaPositiveStorage(domain, operation, owner string, expr *ast.CallExpression) bool {
	if !a.isDefaultArenaDomain(domain) || a.defaultArenaGrows() {
		return true
	}
	if operation == "Alloc" && len(expr.Arguments) == 1 {
		if count, known := a.integerConstantValue(expr.Arguments[0]); known && count != nil && count.Sign() == 0 {
			return true
		}
	}
	target := a.planTargetOS + "/" + a.planTargetArch
	if a.planTargetOS == "" {
		target = "the selected target"
	}
	policy := a.AllocationCapabilities().Profile
	if policy == "" {
		policy = "unresolved"
	}
	a.addErrorAtTokenWithMetadata(expr.Token, "",
		"back the Arena explicitly with Arena.FromBuffer(ref mut buffer[..]), or allocate zero elements; an empty default Arena grows only with a verified allocation provider",
		"Arena.%s on %s needs positive backing, but %s is the empty default Arena and %s (allocation policy %s) declares no verified allocation provider for it to grow from",
		operation, owner, owner, target, policy)
	return false
}
