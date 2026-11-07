package sema

// Canonical storage relation vocabulary (storage.md §§ 12–13, 21) and the
// allocation identities P18 introduces (package18 § 21). They are semantic
// compiler identities, never numeric addresses; zero is absent.
//
// Rules:
//   - rules/memory/storage.md — § 12 "BackingRelation", § 13 "ReclamationAuthority", § 21 "Establish, advance, and end domain"
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 20–31

// BackingRelation describes how a value relates to separate backing storage.
type BackingRelation string

const (
	BackingRelationNone     BackingRelation = "None"
	BackingRelationEmbedded BackingRelation = "Embedded"
	BackingRelationOwned    BackingRelation = "Owned"
	BackingRelationBorrowed BackingRelation = "Borrowed"
)

// ReclamationAuthority identifies which object or domain may release,
// reset or reuse storage.
type ReclamationAuthority string

const (
	ReclamationEnclosingObject ReclamationAuthority = "EnclosingObject"
	ReclamationStaticDomain    ReclamationAuthority = "StaticDomain"
	ReclamationThreadDomain    ReclamationAuthority = "ThreadDomain"
	ReclamationArena           ReclamationAuthority = "Arena"
	ReclamationOwningValue     ReclamationAuthority = "OwningValue"
	ReclamationExternalOwner   ReclamationAuthority = "ExternalOwner"
	ReclamationPlatform        ReclamationAuthority = "Platform"
	ReclamationNone            ReclamationAuthority = "None"
	ReclamationUnknown         ReclamationAuthority = "Unknown"
)

// AllocationIdentityID identifies one allocation-capable producing site's
// backing allocation (package18 § 21).
type AllocationIdentityID uint32

// AllocationContextID identifies one canonical allocation context.
type AllocationContextID uint32

// ReclamationPlanID identifies the exact reclamation paired with an
// allocation whose bytes the owning value reclaims (package18 § 31).
type ReclamationPlanID uint32

func (id AllocationIdentityID) String() string { return identityString("allocation", uint32(id)) }
func (id AllocationContextID) String() string  { return identityString("allocation-context", uint32(id)) }
func (id ReclamationPlanID) String() string    { return identityString("reclamation", uint32(id)) }

func (a *placeIdentityAllocator) allocation(key string) AllocationIdentityID {
	return allocateIdentity(&a.allocations, key, 1)
}

func (a *placeIdentityAllocator) allocationContext(key string) AllocationContextID {
	return allocateIdentity(&a.allocationContexts, key, 1)
}

func (a *placeIdentityAllocator) reclamation(key string) ReclamationPlanID {
	return allocateIdentity(&a.reclamations, key, 1)
}

// reclamationAuthorityOf is the authority over bytes allocated with a
// storage origin: an Arena controls its own reset/release, allocator-backed
// backing is reclaimed by the owning value, and an unknown origin grants no
// reclamation assumption (storage.md § 13; package18 §§ 20, 23).
func reclamationAuthorityOf(origin StorageOrigin) ReclamationAuthority {
	switch origin {
	case StorageOriginArena:
		return ReclamationArena
	case StorageOriginAllocatorBacked:
		return ReclamationOwningValue
	default:
		return ReclamationUnknown
	}
}

// BackingTransitionKind is one canonical storage transition on a backing
// invalidation domain (storage.md § 21; package18 §§ 25–31, 66–67).
type BackingTransitionKind string

const (
	BackingTransitionNone              BackingTransitionKind = "none"
	BackingTransitionEstablishDomain   BackingTransitionKind = "establish-domain"
	BackingTransitionAdvanceEpoch      BackingTransitionKind = "advance-epoch"
	BackingTransitionReplaceDomain     BackingTransitionKind = "replace-domain"
	BackingTransitionEndDomain         BackingTransitionKind = "end-domain"
	BackingTransitionReclaim           BackingTransitionKind = "reclaim"
	BackingTransitionDestroyElements   BackingTransitionKind = "destroy-elements-reverse"
	// BackingTransitionReclaimPerPlan reclaims the bytes only when the
	// backing's allocation plan gives OwningValue authority (§ 30(3)).
	BackingTransitionReclaimPerPlan BackingTransitionKind = "reclaim-per-allocation-plan"
)
