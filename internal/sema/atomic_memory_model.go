package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// AtomicMemoryPath classifies a normal operation outcome. Participation means
// this outcome enters its object's total order, not a compile-time index
// in a chosen execution. Only RMW outcomes read the immediately preceding
// modification; a failed CAS is an ordinary read of its observed state.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–17,24,52(2–4).
type AtomicMemoryPath struct {
	Variant                               string
	Order                                 string
	ReadsStorage                          bool
	ModifiesStorage                       bool
	ReadModifyWrite                       bool
	ParticipatesInModificationOrder       bool
	ReadsImmediatelyPrecedingModification bool
	ReleaseSequence                       AtomicReleaseSequenceEffect
}

// ResolvedAtomicMemoryOperation preserves one validated operation and its
// outcome-sensitive ordering requirements. Storage uses canonical Place and
// referent facts in one provenance snapshot; different unresolved references
// may alias. Absent storage facts mean Unknown, never a fresh atomic identity.
// TotalOrderPerObject and RequiresHappensBeforeCoherence are semantic constraints;
// neither imposes a global modification order nor solves a runtime schedule.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–17,52(1–6).
type ResolvedAtomicMemoryOperation struct {
	// TypeIdentity separates nominal T from its capability carrier. Fence
	// operations have neither. Rules: rules/concurrency/atomics.md §37A(4–6).
	TypeIdentity                   AtomicValueIdentity
	Operation                      string
	Source                         lexer.Token
	Callable                       CallableID
	Storage                        SynchronizationResourceFact
	StorageResolved                bool
	TotalOrderPerObject            bool
	RequiresHappensBeforeCoherence bool
	Paths                          []AtomicMemoryPath
}

// recordAtomicMemoryOperation publishes only the already-validated atomic
// receiver and orders. Canonical referent resolution preserves alias origins;
// no method spelling on an ordinary user type establishes an atomic object.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–17,25–28,52;
// rules/concurrency/atomics.md §§12–25,45–46.
func (a *Analyzer) recordAtomicMemoryOperation(call *ast.CallExpression, member *ast.MemberExpression, order string, element Type) {
	if a.summaryPass || !a.callGraphPathReachable {
		return
	}
	plan := ResolvedAtomicMemoryOperation{Operation: member.Property.Value, Source: member.Property.Token, Callable: a.currentCallable, TotalOrderPerObject: true, RequiresHappensBeforeCoherence: true}
	plan.TypeIdentity = AtomicValueTypeIdentity(element)
	if place, ok := a.resolvePlace(member.Object); ok {
		if place.Type.Kind == ReferenceType && place.Type.Element != nil {
			place = a.canonicalDereferencePlace(member.Object, place, place.Type)
		}
		if isAtomicType(place.Type) {
			plan.Storage = NewSynchronizationResourceFact(place, SynchronizationResourceSemantics{ContractID: "sec.Atomic"})
			plan.Storage.provenance = a.resourceIdentityDomain
			plan.StorageResolved = true
		}
	}
	if plan.Operation == "CompareExchange" {
		cas, ok := a.ResolvedCompareExchangeOf(call)
		if !ok {
			return
		}
		for _, path := range []CompareExchangePath{cas.Success, cas.Failure} {
			plan.Paths = append(plan.Paths, AtomicMemoryPath{Variant: path.Variant, Order: path.Order, ReadsStorage: path.ReadsStorage, ModifiesStorage: path.ModifiesStorage, ReadModifyWrite: path.ReadModifyWrite, ParticipatesInModificationOrder: path.ModifiesStorage, ReadsImmediatelyPrecedingModification: path.ReadModifyWrite})
		}
	} else {
		if order == "" {
			return
		} // No validated order means no successful fact.
		rmw := false
		switch plan.Operation {
		case "Load", "Store":
		case "Swap", "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor":
			rmw = true
		default:
			return // A future operation needs its own canonical memory classification.
		}
		writes := plan.Operation != "Load"
		plan.Paths = []AtomicMemoryPath{{Order: order, ReadsStorage: plan.Operation != "Store", ModifiesStorage: writes, ReadModifyWrite: rmw, ParticipatesInModificationOrder: writes, ReadsImmediatelyPrecedingModification: rmw}}
	}
	for i := range plan.Paths {
		plan.Paths[i].ReleaseSequence = atomicReleaseSequenceEffect(plan.Paths[i])
	}
	if a.resolvedAtomicMemoryOperations == nil {
		a.resolvedAtomicMemoryOperations = map[*ast.CallExpression]ResolvedAtomicMemoryOperation{}
	}
	a.resolvedAtomicMemoryOperations[call] = plan
}

// ResolvedAtomicMemoryOperationOf returns detached outcome/Place/type facts
// without inference, CTE or mutation. Source locations are not runtime order.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–17,52;
// rules/compiler/compiler_analysis.md — "Analysis snapshot consistency".
func (a *Analyzer) ResolvedAtomicMemoryOperationOf(call *ast.CallExpression) (ResolvedAtomicMemoryOperation, bool) {
	if a == nil || call == nil {
		return ResolvedAtomicMemoryOperation{}, false
	}
	plan, ok := a.resolvedAtomicMemoryOperations[call]
	if !ok {
		return ResolvedAtomicMemoryOperation{}, false
	}
	plan.Paths = append([]AtomicMemoryPath(nil), plan.Paths...)
	if plan.StorageResolved {
		fact := NewSynchronizationResourceFact(plan.Storage.Place(), plan.Storage.Semantics())
		fact.provenance = plan.Storage.provenance
		plan.Storage = fact
	}
	return plan, true
}

// AtomicMemoryOperations lists source-ordered detached facts. This deterministic
// presentation order must never be mistaken for an inter-thread total order.
// Rules: rules/concurrency/concurrency_memory_model.md §§16(1–3),52.
func (a *Analyzer) AtomicMemoryOperations() []ResolvedAtomicMemoryOperation {
	if a == nil {
		return nil
	}
	var plans []ResolvedAtomicMemoryOperation
	for call := range a.resolvedAtomicMemoryOperations {
		plan, _ := a.ResolvedAtomicMemoryOperationOf(call)
		plans = append(plans, plan)
	}
	sort.Slice(plans, func(i, j int) bool {
		l, r := plans[i], plans[j]
		if l.Source.File != r.Source.File {
			return l.Source.File < r.Source.File
		}
		if l.Source.ByteStart != r.Source.ByteStart {
			return l.Source.ByteStart < r.Source.ByteStart
		}
		return l.Callable < r.Callable
	})
	return plans
}

// AtomicModificationOrderRelationship reuses the canonical four-state identity
// relation in a caller-proven common ownership/execution state. The caller
// must normalize moves, replacement and execution instances before supplying
// state, exactly as for other synchronization-resource observations. Merely
// sharing an Analyzer is not proof of object lifetime continuity.
// Same means one modification-order domain; Disjoint means separate
// domains, while unresolved aliases remain MayAlias/Unknown. A read and a
// modification can share a domain without both entering its modification order.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–17,52(2);
// rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity".
func AtomicModificationOrderRelationship(left, right ResolvedAtomicMemoryOperation, state *SynchronizationResourceSnapshot) SynchronizationResourceRelationship {
	if state == nil || !left.StorageResolved || !right.StorageResolved || left.Storage.provenance == nil || left.Storage.provenance != right.Storage.provenance {
		return ResourceUnknown
	}
	return SynchronizationRelationship(state.Identify(left.Storage), state.Identify(right.Storage))
}
