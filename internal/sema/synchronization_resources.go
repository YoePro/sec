package sema

import (
	"math/big"

	"sec/internal/ast"
)

// SynchronizationResourceSemantics identifies a compiler-known or explicitly
// contracted synchronization resource, independently of source binding names.
// Capacity nil and Reentrancy empty retain missing contract information; neither
// is implicitly the exclusive, non-reentrant mutex contract.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource
// identity", "Resource capacity", "Reentrant resources".
type SynchronizationResourceSemantics struct {
	ContractID string
	Capacity   *big.Int
	Reentrancy ResourceReentrancy
}

type ResourceReentrancy string

const (
	ResourceNonReentrant ResourceReentrancy = "non-reentrant"
	ResourceReentrant    ResourceReentrancy = "reentrant"
)

// SynchronizationResourceFact retains canonical provenance at an observation
// point. It does not assert that a storage path keeps the same resource after
// an ownership transfer, replacement, or another execution's mutation.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource
// identity", "Ownership and resource transfer"; rules/concurrency/mutex.md — §1(2), §5(3–4).
type SynchronizationResourceFact struct {
	place     Place
	semantics SynchronizationResourceSemantics
	// Analyzer observations retain the namespace in which RootID was assigned.
	// Imported/rebound facts require a producer-owned canonical namespace.
	provenance *SynchronizationResourceSnapshot
}

// NewSynchronizationResourceFact consumes producer-owned canonical Place and
// resource contract facts. The Place must denote the resource itself, rather
// than the storage of a possibly copied handle. A contract describes resource semantics,
// rather than being inferred from a variable or method name.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource
// identity", "Resource capacity".
func NewSynchronizationResourceFact(place Place, semantics SynchronizationResourceSemantics) SynchronizationResourceFact {
	return SynchronizationResourceFact{place: snapshotResourcePlace(place), semantics: cloneResourceSemantics(semantics)}
}

// Place returns detached provenance for downstream ownership/flow producers.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity".
func (f SynchronizationResourceFact) Place() Place { return snapshotResourcePlace(f.place) }

// Semantics returns the exact resource contract without exposing mutable facts.
// Rules: rules/analysis/deadlock_analysis.md — "Resource capacity", "Reentrant resources".
func (f SynchronizationResourceFact) Semantics() SynchronizationResourceSemantics {
	return cloneResourceSemantics(f.semantics)
}

// snapshotResourcePlace detaches structural type, exact index and origin data.
// Rules: rules/compiler/compiler_analysis.md — immutable analysis results;
// rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity".
func snapshotResourcePlace(place Place) Place {
	result := place
	result.Type = semanticSnapshotType(place.Type)
	result.Projections = clonePlaceProjections(place.Projections)
	result.AlternativeOrigins = make([]Place, len(place.AlternativeOrigins))
	for i, alternative := range place.AlternativeOrigins {
		result.AlternativeOrigins[i] = snapshotResourcePlace(alternative)
	}
	return result
}

// cloneResourceSemantics preserves exact arbitrary-precision capacity facts.
// Rules: rules/analysis/deadlock_analysis.md — "Resource capacity".
func cloneResourceSemantics(semantics SynchronizationResourceSemantics) SynchronizationResourceSemantics {
	if semantics.Capacity != nil {
		semantics.Capacity = new(big.Int).Set(semantics.Capacity)
	}
	return semantics
}

// SynchronizationResourceSnapshot scopes Place identities to one canonical
// provenance/ownership state. Producers must normalize referents and ownership
// transfers in that state before identifying resources. Different states and
// independent analyzers cannot prove identity merely by reusing a root number.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource
// identity", "Ownership and resource transfer"; rules/mlir/semantic-ir/sec_semantic_ir_place_reference_v1.md — §§2, 13.
type SynchronizationResourceSnapshot struct{ identity byte }

// NewSynchronizationResourceSnapshot creates an independent identity domain.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity".
func NewSynchronizationResourceSnapshot() *SynchronizationResourceSnapshot {
	return &SynchronizationResourceSnapshot{}
}

// SynchronizationResourceIdentity is an immutable, snapshot-qualified resource
// identity. Its zero value expresses unresolved identity, never a named lock.
type SynchronizationResourceIdentity struct {
	snapshot *SynchronizationResourceSnapshot
	fact     SynchronizationResourceFact
}

// Identify qualifies canonical provenance in this snapshot. Observations from
// different points must first be rebound by their owning flow/ownership producer;
// this service does not silently treat storage reuse as resource continuity.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity", "Ownership and resource transfer".
func (s *SynchronizationResourceSnapshot) Identify(fact SynchronizationResourceFact) SynchronizationResourceIdentity {
	return SynchronizationResourceIdentity{snapshot: s, fact: fact}
}

type SynchronizationResourceRelationship string

const (
	ResourceSame     SynchronizationResourceRelationship = "Same"
	ResourceDisjoint SynchronizationResourceRelationship = "Disjoint"
	ResourceMayAlias SynchronizationResourceRelationship = "MayAlias"
	ResourceUnknown  SynchronizationResourceRelationship = "Unknown"
)

// SynchronizationRelationship retains four-state identity precision. It uses
// the shared Place relationship service only after checking root identity and
// unresolved referent provenance, and never uses display names as proof.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity";
// rules/mlir/semantic-ir/sec_semantic_ir_place_reference_v1.md — §5 "Place relationships".
func SynchronizationRelationship(left, right SynchronizationResourceIdentity) SynchronizationResourceRelationship {
	if left.snapshot == nil || left.snapshot != right.snapshot || left.fact.provenance != right.fact.provenance || !compatibleResourceSemantics(left.fact.semantics, right.fact.semantics) {
		return ResourceUnknown
	}
	return resourcePlaceRelationship(left.fact.place, right.fact.place)
}

// compatibleResourceSemantics refuses conflicting or missing contracts rather
// than proving disjointness from different resource-kind names.
// Rules: rules/analysis/deadlock_analysis.md — "Resource capacity", "Reentrant resources".
func compatibleResourceSemantics(left, right SynchronizationResourceSemantics) bool {
	if left.ContractID == "" || left.ContractID != right.ContractID || left.Reentrancy != right.Reentrancy {
		return false
	}
	if left.Capacity == nil || right.Capacity == nil {
		return left.Capacity == nil && right.Capacity == nil
	}
	return left.Capacity.Sign() > 0 && right.Capacity.Sign() > 0 && left.Capacity.Cmp(right.Capacity) == 0
}

// resourcePlaceRelationship joins finite alternative origins conservatively:
// agreement retains proof, disagreement admits aliases, missing proof is Unknown.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity";
// rules/mlir/packages/sec-mlir-dialect_package15.md — §20 "Alternative origins".
func resourcePlaceRelationship(left, right Place) SynchronizationResourceRelationship {
	if left.AmbiguousProvenance || right.AmbiguousProvenance {
		return ResourceUnknown
	}
	joined := SynchronizationResourceRelationship("")
	for _, l := range placeOriginAlternatives(left) {
		for _, r := range placeOriginAlternatives(right) {
			next := singleResourcePlaceRelationship(l, r)
			if next == ResourceUnknown {
				return ResourceUnknown
			}
			if joined == "" {
				joined = next
			} else if joined != next {
				joined = ResourceMayAlias
			}
		}
	}
	return joined
}

// singleResourcePlaceRelationship rejects name fallback and keeps unresolved
// reference roots potentially aliased even when their binding IDs differ.
// Properties and unnormalized slices lack precise resource provenance.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity";
// rules/mlir/semantic-ir/sec_semantic_ir_place_reference_v1.md — §§5, 6, 14.
func singleResourcePlaceRelationship(left, right Place) SynchronizationResourceRelationship {
	for _, place := range []Place{left, right} {
		if place.RootID == 0 || place.AmbiguousProvenance || place.Type.Kind == ReferenceType {
			return ResourceUnknown
		}
		switch place.RootKind {
		case PlaceRootLocal, PlaceRootParameter, PlaceRootReceiver, PlaceRootStatic, PlaceRootDeref:
		default:
			return ResourceUnknown
		}
		for _, projection := range place.Projections {
			switch projection.Kind {
			case PlaceField, PlaceIndex, PlaceUnionPayload:
			case PlaceDereference:
				if place.RootKind != PlaceRootDeref {
					return ResourceUnknown
				}
			default:
				return ResourceUnknown
			}
		}
	}
	if left.RootID != right.RootID && (left.RootKind == PlaceRootDeref || right.RootKind == PlaceRootDeref) {
		return ResourceMayAlias
	}
	if left.RootID == right.RootID && left.RootKind != right.RootKind {
		return ResourceUnknown
	}
	// Relationship's legacy presentation fallback must never influence resource
	// identity. Canonical root IDs and projection facts are the only proof input.
	left.Root, right.Root = "resource", "resource"
	switch Relationship(left, right) {
	case PlaceSame:
		return ResourceSame
	case PlaceDisjoint:
		return ResourceDisjoint
	case PlaceContains, PlaceContainedBy, PlacePotentiallyOverlapping:
		return ResourceMayAlias
	default:
		return ResourceUnknown
	}
}

// resolvePlace records compiler-known resource provenance while retaining the
// existing Place resolver's behavior. A safe reference is normalized through
// the shared referent producer; getters remain imprecise resource origins.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity";
// rules/concurrency/mutex.md — §§1(2), 2(1), 5(3–4);
// rules/mlir/semantic-ir/sec_semantic_ir_place_reference_v1.md — §14 "Place dereference".
func (a *Analyzer) resolvePlace(expr ast.Expression) (Place, bool) {
	place, ok := a.resolvePlacePath(expr)
	if ok {
		resourcePlace := place
		if place.Type.Kind == ReferenceType && place.Type.Element != nil && isMutexType(*place.Type.Element) {
			resourcePlace = a.canonicalDereferencePlace(expr, place, place.Type)
		}
		if isMutexType(resourcePlace.Type) {
			if a.synchronizationResources == nil {
				a.synchronizationResources = map[ast.Expression]SynchronizationResourceFact{}
			}
			if a.resourceIdentityDomain == nil {
				a.resourceIdentityDomain = NewSynchronizationResourceSnapshot()
			}
			fact := NewSynchronizationResourceFact(resourcePlace, SynchronizationResourceSemantics{
				ContractID: "sec.Mutex", Capacity: big.NewInt(1), Reentrancy: ResourceNonReentrant,
			})
			fact.provenance = a.resourceIdentityDomain
			a.synchronizationResources[expr] = fact
		}
	}
	return place, ok
}

// SynchronizationResourceOf returns an immutable resource observation produced
// during canonical Place resolution. It is not an acquisition/held-state fact.
// Rules: rules/analysis/deadlock_analysis.md — "Canonical synchronization-resource identity".
func (a *Analyzer) SynchronizationResourceOf(expr ast.Expression) (SynchronizationResourceFact, bool) {
	if a == nil {
		return SynchronizationResourceFact{}, false
	}
	fact, ok := a.synchronizationResources[expr]
	return fact, ok
}
