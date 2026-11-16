package semantic

import (
	"fmt"
	"slices"
	"strings"
)

// ArenaParentDependency records the backing dependency transferred into a
// borrowed child Arena. Storage is a canonical Sema storage identity, not an
// address or local-slot ID. Reference and bounds reuse P15/P16 vocabulary;
// DynamicRange names a resolved range evaluation, not a guessed byte extent.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§49–50,97;
// rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §14.
type ArenaParentDependency struct {
	Storage      uint32
	Reference    ReferenceMetadata
	StaticRange  *StaticSliceRange
	DynamicRange SliceRangeID
}

// verifyArenaParent checks that a nested child retains the actual incoming
// borrow's storage/domain/epoch, with explicit bounds and a distinct domain.
// It never creates a parent identity or infers range/lifetime facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§50,97;
// rules/memory/arena.md — §51(2)–(6).
func verifyArenaParent(op Operation, source *ReferenceMetadata, bounds *SliceFacts) error {
	var parent *ArenaParentDependency
	if op.Arena != nil {
		parent = op.Arena.Parent
	}
	nested := op.Kind == OpArenaCreateBorrowed && source != nil && source.Provenance == "arena"
	if !nested && parent == nil {
		return nil
	}
	if !nested || parent == nil {
		return fmt.Errorf("nested Arena requires its transferred parent dependency on borrowed creation")
	}
	if parent.Storage == 0 || !slices.Contains(source.OriginStorages, parent.Storage) || !sameSliceValidityFacts(source, &parent.Reference) {
		return fmt.Errorf("nested Arena parent storage/reference facts disagree with its backing borrow")
	}
	if parent.Reference.Validity != ReferenceDynamicEpoch || parent.Reference.InvalidationDomain == 0 || parent.Reference.EpochDependency == 0 || parent.Reference.InvalidationDomain == op.Backing.Domain {
		return fmt.Errorf("nested Arena requires distinct parent/child domains and the parent epoch")
	}
	if (parent.StaticRange == nil) == (parent.DynamicRange == 0) {
		return fmt.Errorf("nested Arena requires exactly one resolved static or dynamic borrow range")
	}
	if r := parent.StaticRange; r != nil && (r.Start == nil || r.EndExclusive == nil || r.Start.Sign() < 0 || r.Start.Cmp(r.EndExclusive) > 0) {
		return fmt.Errorf("nested Arena has invalid half-open backing borrow bounds")
	}
	if bounds != nil {
		if r := bounds.StaticRange; r != nil {
			actual := parent.StaticRange
			if actual == nil || actual.Start.Cmp(r.Start) != 0 || actual.EndExclusive.Cmp(r.EndExclusive) != 0 {
				return fmt.Errorf("nested Arena changes the transferred static borrow bounds")
			}
		}
		if bounds.DynamicRange != 0 && parent.DynamicRange != bounds.DynamicRange {
			return fmt.Errorf("nested Arena changes the transferred dynamic borrow range")
		}
	}
	if op.Backing.Reclamation != 0 {
		return fmt.Errorf("nested Arena borrows parent bytes without an individual reclamation plan")
	}
	return nil
}

// verifyNestedArenaLiveness tracks may-live children to a CFG fixed point.
// Joining paths retains every possibly live dependency. Child Reset does not
// release backing; terminal child Release/Destroy ends that dependency. Parent
// allocation preserves it, while parent invalidation/reclamation is forbidden.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§50,97;
// rules/memory/arena.md — §51(4)–(7).
func verifyNestedArenaLiveness(fn *Function, blocks map[BlockID]*Block) error {
	parents := map[uint32]uint32{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpArenaCreateBorrowed && op.Arena.Parent != nil {
				parents[op.Backing.Domain] = op.Arena.Parent.Reference.InvalidationDomain
			}
		}
	}
	if len(parents) == 0 {
		return nil
	}
	incoming := map[BlockID]map[uint32]bool{fn.Entry: {}}
	pending := []BlockID{fn.Entry}
	processed := map[BlockID]bool{}
	for len(pending) > 0 {
		id := pending[0]
		pending = pending[1:]
		live := map[uint32]bool{}
		for child := range incoming[id] {
			live[child] = true
		}
		processed[id] = true
		for _, op := range blocks[id].Operations {
			if op.Backing != nil {
				domain := op.Backing.Domain
				invalidates := op.Kind == OpArenaReset || op.Kind == OpArenaRelease || op.Kind == OpArenaDestroy || op.Kind == OpStorageAdvanceEpoch || op.Kind == OpStorageEndDomain || op.Kind == OpStorageReclaim
				if invalidates {
					for child := range live {
						if parents[child] == domain {
							return fmt.Errorf("%s invalidates parent domain #%d while a child Arena remains live", op.Kind, domain)
						}
					}
				}
				if op.Kind == OpArenaCreateBorrowed && op.Arena.Parent != nil {
					live[domain] = true
				}
				if op.Kind == OpArenaRelease || op.Kind == OpArenaDestroy {
					delete(live, domain)
				}
			}
			for _, target := range op.Successors {
				next, exists := incoming[target.Block]
				if !exists {
					next = map[uint32]bool{}
					incoming[target.Block] = next
				}
				changed := false
				for child := range live {
					if !next[child] {
						next[child] = true
						changed = true
					}
				}
				if !processed[target.Block] || changed {
					pending = append(pending, target.Block)
				}
			}
		}
	}
	return nil
}

// formatArenaParent preserves deterministic logical dependency and byte-borrow
// bounds without selecting physical parent storage or embedding addresses.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §97;
// rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §27.
func formatArenaParent(out *strings.Builder, parent *ArenaParentDependency) {
	if parent == nil {
		return
	}
	fmt.Fprintf(out, " parent-storage=#%d", parent.Storage)
	formatReferenceMetadata(out, &parent.Reference)
	if parent.StaticRange != nil {
		fmt.Fprintf(out, " parent-range=[%s,%s)", parent.StaticRange.Start, parent.StaticRange.EndExclusive)
	} else {
		fmt.Fprintf(out, " parent-range=dynamic#%d", parent.DynamicRange)
	}
}
