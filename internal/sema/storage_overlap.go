package sema

// StorageOverlap keeps an established storage relationship separate from an
// alias possibility. Its zero value is uncertainty, never a safety proof.
// This fact alone establishes neither concurrency nor a data race.
// Rules: rules/analysis/data_races.md — "Storage overlap".
type StorageOverlap uint8

const (
	StorageMayOverlap StorageOverlap = iota
	StorageDisjoint
	StorageOverlapping
)

// StorageOverlapSnapshot qualifies canonical Place identities in one shared
// provenance state. Producers must normalize referents and rebind storage
// identities across ownership, generation, or execution-state changes before
// comparing them. Root numbers from independent analyses are not identities.
// Rules: rules/analysis/data_races.md — "Canonical Place and provenance",
// "Raw pointers and unsafe code"; rules/mlir/semantic-ir/sec_semantic_ir_place_reference_v1.md — §§2, 13.
type StorageOverlapSnapshot struct{ identity byte }

// NewStorageOverlapSnapshot creates a distinct canonical identity scope.
// Rules: rules/analysis/data_races.md — "Canonical Place and provenance".
func NewStorageOverlapSnapshot() *StorageOverlapSnapshot { return &StorageOverlapSnapshot{} }

// StorageLocation is a detached, snapshot-qualified observation of accessed
// storage. Its zero value carries no proven origin.
// Rules: rules/analysis/data_races.md — "Canonical Place and provenance".
type StorageLocation struct {
	snapshot *StorageOverlapSnapshot
	place    Place
}

// Location consumes an already resolved canonical storage path, without AST
// rediscovery or inference from source spelling. The caller owns normalization
// into this snapshot; passing a reference holder does not identify its referent.
// Rules: rules/analysis/data_races.md — "Canonical Place and provenance".
func (s *StorageOverlapSnapshot) Location(place Place) StorageLocation {
	// The canonical representation is a flat origin set. Do not let the
	// compatibility clone discard an unnormalized nested alias alternative.
	for _, alternative := range place.AlternativeOrigins {
		if len(alternative.AlternativeOrigins) != 0 {
			place.AmbiguousProvenance = true
		}
	}
	return StorageLocation{snapshot: s, place: clonePlace(place)}
}

// ClassifyStorageOverlap joins canonical origin alternatives without turning
// possible aliasing into established overlap. Only unanimous proofs survive;
// missing provenance or incompatible snapshots retain MayOverlap. No race
// diagnostic or execution-overlap fact is synthesized by this query.
// Rules: rules/analysis/data_races.md — "Storage overlap", "Raw pointers and unsafe code".
func ClassifyStorageOverlap(left, right StorageLocation) StorageOverlap {
	if left.snapshot == nil || left.snapshot != right.snapshot || left.place.AmbiguousProvenance || right.place.AmbiguousProvenance {
		return StorageMayOverlap
	}
	var result StorageOverlap
	first := true
	for _, l := range placeOriginAlternatives(left.place) {
		for _, r := range placeOriginAlternatives(right.place) {
			next := singleStorageOverlap(l, r)
			if next == StorageMayOverlap || !first && next != result {
				return StorageMayOverlap
			}
			result, first = next, false
		}
	}
	return result
}

// singleStorageOverlap delegates structural proofs to the shared Place
// service, excluding its legacy name fallback and union-arm disjointness:
// mutually exclusive active variants do not prove disjoint physical storage
// between concurrent observations. Unresolved indirection likewise cannot
// prove either identity or disjointness from its holder's binding ID.
// Rules: rules/analysis/data_races.md — "Storage overlap", "Union payloads";
// rules/mlir/semantic-ir/sec_semantic_ir_place_reference_v1.md — §5 "Place relationships".
func singleStorageOverlap(left, right Place) StorageOverlap {
	for _, place := range []Place{left, right} {
		if place.RootID == 0 || place.AmbiguousProvenance || len(place.AlternativeOrigins) != 0 || place.Type.Kind == ReferenceType {
			return StorageMayOverlap
		}
		switch place.RootKind {
		case PlaceRootLocal, PlaceRootParameter, PlaceRootReceiver, PlaceRootStatic:
		default:
			return StorageMayOverlap
		}
		for _, projection := range place.Projections {
			switch projection.Kind {
			case PlaceField, PlaceIndex:
			case PlaceSlice:
				if projection.SliceStart == nil || projection.SliceEnd == nil || projection.SliceStart.Sign() < 0 || projection.SliceEnd.Cmp(projection.SliceStart) < 0 {
					return StorageMayOverlap
				}
			default:
				return StorageMayOverlap
			}
		}
	}
	if left.RootID == right.RootID && left.RootKind != right.RootKind {
		return StorageMayOverlap
	}
	// IDs are authoritative even if presentation is absent or has changed.
	left.Root, right.Root = "storage", "storage"
	switch Relationship(left, right) {
	case PlaceDisjoint:
		return StorageDisjoint
	case PlaceSame, PlaceContains, PlaceContainedBy:
		return StorageOverlapping
	default:
		return StorageMayOverlap
	}
}
