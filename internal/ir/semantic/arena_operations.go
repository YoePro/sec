package semantic

import (
	"fmt"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/sema"
)

// Arena operations preserve logical storage transitions without choosing an ABI.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§86–96.
const (
	OpArenaCreateBorrowed   OpKind = "arena.create-borrowed"
	OpArenaCreateOwnedFixed OpKind = "arena.create-owned-fixed"
	OpArenaCreateGrowable   OpKind = "arena.create-growable"
	// OpArenaCreateDefault is the MD-136 empty default Arena: no operand,
	// no backing, zero capacity, allocation-free (arena.md § 131).
	OpArenaCreateDefault OpKind = "arena.create-default"
	OpArenaNew           OpKind = "arena.new"
	OpArenaAlloc         OpKind = "arena.alloc"
	OpArenaReset         OpKind = "arena.reset"
	OpArenaRelease       OpKind = "arena.release"
	OpArenaDestroy       OpKind = "arena.destroy"
)

// ArenaOperationFacts retains already resolved plan facts. State versions order
// mutations; epochs validate references. Neither is a physical cursor. Domain,
// origin, backing relation, authority and transition reuse BackingMetadata and
// the canonical storage vocabulary; these records allocate no new identities.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§70–75,86–96;
// rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §§4–13,23,27.
type ArenaOperationFacts struct {
	// Parent retains nested borrowed backing; only borrowed creation sets it.
	Parent                     *ArenaParentDependency
	StateBefore, StateAfter    uint32
	EpochBefore, EpochAfter    uint32
	BackingKind, GrowthKind    string
	ProviderPlan, CapacityPlan string
	LayoutPlan, DefaultPlan    string
	DependencyProof            string
	RetainedBorrow             uint32
	FailureAtomic              bool
	// EpochAdvance is the Reset epoch exhaustion fact (MD-140); zero for
	// every other operation.
	EpochAdvance ArenaEpochAdvanceFacts
}

// ArenaEpochAdvanceFacts records how a Reset advances its retained validity
// epoch, as fixed by Sema: the form (checked, proven-bounded or
// metadata-eliminated), the proof provenance of an elimination, the logical
// width, the single panic outcome and the registered exhaustion reason. The
// Semantic IR epoch is logical, so a checked advance's terminal exhaustion
// path is this op's registered panic; its physical test is materialized by
// lowering before any part of the transition (semantic_ir.md § 51(7)).
// Rules: rules/compiler/semantic_ir.md — § 51(6)-(8);
// rules/memory/arena.md — §§ 39(1), 39(5)-(9), 42(5).
type ArenaEpochAdvanceFacts struct {
	Form             string
	Proof            string
	LogicalEpochBits uint16
	Exhaustion       string
	PanicReasonID    uint32
	PanicReason      string
}

// verifyArenaEpochAdvance enforces semantic_ir.md § 51(8) on a Reset: a known
// form; a checked advance carries no proof while an elimination needs proof
// provenance; a valid logical width; the panic outcome only (never rekey,
// recreation, retirement or fallible Reset); and exactly the registered
// panic.arena-epoch-exhaustion reason. Other operations carry no such fact.
// Rules: rules/compiler/semantic_ir.md — § 51(6)-(8); rules/errors/panic.md — § 13(8).
func verifyArenaEpochAdvance(op Operation) error {
	e := op.Arena.EpochAdvance
	if op.Kind != OpArenaReset {
		if e != (ArenaEpochAdvanceFacts{}) {
			return fmt.Errorf("%s carries a Reset epoch-advance fact", op.Kind)
		}
		return nil
	}
	switch e.Form {
	case string(sema.ArenaEpochAdvanceChecked):
		if e.Proof != "" {
			return fmt.Errorf("arena.reset checked advance carries an elimination proof")
		}
	case string(sema.ArenaEpochAdvanceProvenBounded), string(sema.ArenaEpochAdvanceMetadataEliminated):
		if e.Proof == "" {
			return fmt.Errorf("arena.reset %s advance requires proof provenance", e.Form)
		}
	default:
		return fmt.Errorf("arena.reset requires a resolved epoch-advance form, got %q", e.Form)
	}
	switch e.LogicalEpochBits {
	case 8, 16, 32, 64:
	default:
		return fmt.Errorf("arena.reset requires a resolved logical epoch width, got %d", e.LogicalEpochBits)
	}
	if e.Exhaustion != string(layout.ArenaEpochPanic) {
		return fmt.Errorf("arena.reset exhaustion outcome must be panic, got %q", e.Exhaustion)
	}
	definition, ok := diagnostics.PanicReasonByID(diagnostics.PanicReasonArenaEpochExhaustion)
	if !ok || e.PanicReasonID != uint32(definition.ID) || e.PanicReason != definition.Name {
		return fmt.Errorf("arena.reset exhaustion must use the registered panic.arena-epoch-exhaustion reason, got %d %q", e.PanicReasonID, e.PanicReason)
	}
	return nil
}

// isArenaOperation recognizes only the eight owner operations, not context
// production or source readiness. Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§86–96.
func isArenaOperation(kind OpKind) bool {
	switch kind {
	case OpArenaCreateBorrowed, OpArenaCreateOwnedFixed, OpArenaCreateGrowable, OpArenaCreateDefault,
		OpArenaNew, OpArenaAlloc, OpArenaReset, OpArenaRelease, OpArenaDestroy:
		return true
	}
	return false
}

// arenaCheckedOperation identifies candidate/failed/AllocationError operations.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§90–93.
func arenaCheckedOperation(kind OpKind) bool {
	return kind == OpArenaCreateOwnedFixed || kind == OpArenaCreateGrowable || kind == OpArenaNew || kind == OpArenaAlloc
}

// verifyArenaOperation enforces the typed, Place-based contracts and explicit
// state/epoch effects. It consumes resolved proof/plan references, never infers
// layout, provider, dependency legality or source initialization decisions.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§86–96;
// rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §§6–13,23–24.
func verifyArenaOperation(module *Module, op Operation, values map[ValueID]Value) error {
	f, b := op.Arena, op.Backing
	if f == nil || b == nil || b.Domain == 0 || len(op.Successors) != 0 {
		return fmt.Errorf("%s requires Arena facts, a canonical domain and no successors", op.Kind)
	}
	types := module.Types
	create := op.Kind == OpArenaCreateBorrowed || op.Kind == OpArenaCreateOwnedFixed || op.Kind == OpArenaCreateGrowable || op.Kind == OpArenaCreateDefault
	terminal := op.Kind == OpArenaRelease || op.Kind == OpArenaDestroy
	if f.StateAfter == 0 || create && f.StateBefore != 0 || !create && (f.StateBefore == 0 || f.StateAfter == f.StateBefore) {
		return fmt.Errorf("%s requires its resolved state-version transition", op.Kind)
	}
	if f.BackingKind != "borrowed" && f.BackingKind != "owned" && f.BackingKind != "none" || f.GrowthKind != "fixed" && f.GrowthKind != "growable-nonrelocating" {
		return fmt.Errorf("%s requires resolved backing and growth policies", op.Kind)
	}
	if f.BackingKind == "borrowed" && (f.GrowthKind != "fixed" || b.Relation != string(sema.BackingRelationBorrowed) || f.RetainedBorrow == 0) ||
		f.BackingKind == "owned" && (b.Relation != string(sema.BackingRelationOwned) || f.ProviderPlan == "") ||
		f.BackingKind == "none" && (b.Relation != string(sema.BackingRelationOwned) || f.RetainedBorrow != 0 || f.ProviderPlan == "") {
		return fmt.Errorf("%s requires its backing borrow or resolved provider plan", op.Kind)
	}
	if b.Origin == "" || b.Authority == "" || b.Stability == "" || b.Space == "" {
		return fmt.Errorf("%s requires canonical storage classification", op.Kind)
	}
	transition := string(sema.BackingTransitionNone)
	switch {
	case create:
		transition = string(sema.BackingTransitionEstablishDomain)
		if f.EpochBefore != 0 || f.EpochAfter == 0 {
			return fmt.Errorf("%s establishes an initial epoch", op.Kind)
		}
	case terminal:
		transition = string(sema.BackingTransitionEndDomain)
		if f.EpochBefore == 0 || f.EpochAfter != 0 {
			return fmt.Errorf("%s ends its domain without publishing another epoch", op.Kind)
		}
	case op.Kind == OpArenaReset:
		transition = string(sema.BackingTransitionAdvanceEpoch)
		if f.EpochBefore == 0 || f.EpochAfter == 0 || f.EpochAfter == f.EpochBefore {
			return fmt.Errorf("arena.reset advances the validity epoch")
		}
	default:
		if f.EpochBefore == 0 || f.EpochBefore != f.EpochAfter {
			return fmt.Errorf("%s preserves the validity epoch", op.Kind)
		}
	}
	if err := verifyArenaEpochAdvance(op); err != nil {
		return err
	}
	if b.Transition != transition {
		return fmt.Errorf("%s requires canonical storage transition %s", op.Kind, transition)
	}
	if arenaCheckedOperation(op.Kind) {
		if len(op.Results) != 3 || !typeHasKind(types, op.Results[1].Type, TypeBool) || !isAllocationErrorType(types, op.Results[2].Type) || !f.FailureAtomic {
			return fmt.Errorf("%s produces candidate, failed and core AllocationError with atomic failure", op.Kind)
		}
	} else if create {
		if len(op.Results) != 1 {
			return fmt.Errorf("arena.create-borrowed produces one owned Arena")
		}
	} else if len(op.Results) != 0 {
		return fmt.Errorf("%s has no result", op.Kind)
	}
	if op.Kind == OpArenaCreateDefault {
		// MD-136: an owned empty Arena from no operand, no backing.
		if !typeHasKind(types, op.Results[0].Type, TypeArena) || op.Results[0].Ownership != OwnershipOwned || len(op.Operands) != 0 ||
			f.BackingKind != "none" || f.CapacityPlan != "empty-default" {
			return fmt.Errorf("arena.create-default produces an owned empty Arena with no backing")
		}
		return nil
	}
	if create {
		if !typeHasKind(types, op.Results[0].Type, TypeArena) || op.Results[0].Ownership != OwnershipOwned || len(op.Operands) != 1 {
			return fmt.Errorf("%s produces an owned Arena from one capacity/backing operand", op.Kind)
		}
		if op.Kind == OpArenaCreateBorrowed {
			slice, ok := types.Lookup(values[op.Operands[0]].Type)
			if !ok || slice.Kind != TypeMutableSlice || !typeHasKind(types, slice.Element, TypeByte) || values[op.Operands[0]].Ownership != OwnershipMutableReference || f.BackingKind != "borrowed" {
				return fmt.Errorf("arena.create-borrowed transfers one mutable byte-slice borrow")
			}
		} else {
			if !arenaUintOperand(types, values[op.Operands[0]].Type) || f.BackingKind != "owned" || f.CapacityPlan == "" {
				return fmt.Errorf("%s requires uint byte capacity and a resolved capacity plan", op.Kind)
			}
			growable := op.Kind == OpArenaCreateGrowable
			if growable != (f.GrowthKind == "growable-nonrelocating") {
				return fmt.Errorf("%s has inconsistent growth policy", op.Kind)
			}
		}
		return nil
	}
	if terminal {
		if len(op.Operands) != 1 || !typeHasKind(types, values[op.Operands[0]].Type, TypeArena) || values[op.Operands[0]].Ownership != OwnershipOwned || f.DependencyProof == "" {
			return fmt.Errorf("%s consumes one owned Arena with resolved dependency legality", op.Kind)
		}
		if op.Kind == OpArenaDestroy {
			if op.Ownership == nil || !oneOf(op.Ownership.Cause, ownDestroyCauses) {
				return fmt.Errorf("arena.destroy requires its P17 destruction cause and plan")
			}
			plan, err := destructionPlan(module, op.Ownership.DestructionPlan, values[op.Operands[0]].Type)
			if err != nil {
				return err
			}
			if plan.Kind != "compiler-known-resource" || plan.Resource != "Arena" {
				return fmt.Errorf("arena.destroy requires the Arena resource destruction plan")
			}
		}
		return nil
	}
	wantOperands := 1
	if op.Kind == OpArenaAlloc {
		wantOperands = 2
	}
	if len(op.Operands) != wantOperands {
		return fmt.Errorf("%s has invalid operand count", op.Kind)
	}
	place, ok := placeType(types, values[op.Operands[0]].Type)
	if !ok || !place.Mutable || !typeHasKind(types, place.Element, TypeArena) {
		return fmt.Errorf("%s requires a writable Place of Arena", op.Kind)
	}
	if op.Kind == OpArenaReset {
		if f.DependencyProof == "" {
			return fmt.Errorf("arena.reset requires resolved dependency legality")
		}
		return nil
	}
	if op.Kind == OpArenaAlloc && !arenaUintOperand(types, values[op.Operands[1]].Type) {
		return fmt.Errorf("arena.alloc count must be uint elements")
	}
	result, ok := types.Lookup(op.Results[0].Type)
	wantKind := TypeReference
	if op.Kind == OpArenaAlloc {
		wantKind = TypeMutableSlice
	}
	if !ok || result.Kind != wantKind || op.Kind == OpArenaNew && !result.Mutable || op.Results[0].Ownership != OwnershipMutableReference {
		return fmt.Errorf("%s produces a mutable direct reference/slice candidate", op.Kind)
	}
	// DefaultPlan is the resolved defaultability/initialization decision, not
	// an implicit scalar zero; aggregate defaults remain owned by that plan.
	if !trivialDestruction(module, result.Element) || f.LayoutPlan == "" || f.DefaultPlan == "" || f.CapacityPlan == "" {
		return fmt.Errorf("%s requires safe defaultable trivially-destructible T and layout/default/capacity plans", op.Kind)
	}
	if err := verifyReferenceMetadata(op); err != nil {
		return err
	}
	if op.Reference.Validity != ReferenceDynamicEpoch || op.Reference.InvalidationDomain != b.Domain || op.Reference.EpochDependency != f.EpochAfter || op.Reference.Provenance != "arena" {
		return fmt.Errorf("%s candidate must retain its Arena domain and current epoch", op.Kind)
	}
	return nil
}

// verifyArenaCandidates prevents publishing a failed candidate. The operation
// immediately branches on failed; all candidate uses must be dominated by the
// success edge, which has one predecessor. No allocation state is committed on
// the failure edge. Source builders supply these branches in P19-20.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§35,90–93.
func verifyArenaCandidates(fn *Function, blocks map[BlockID]*Block, dom map[BlockID]map[BlockID]bool) error {
	preds := map[BlockID]int{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, target := range op.Successors {
				preds[target.Block]++
			}
		}
	}
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			if !arenaCheckedOperation(op.Kind) {
				continue
			}
			if index+2 != len(block.Operations) {
				return fmt.Errorf("%s must immediately branch on failed", op.Kind)
			}
			branch := block.Operations[index+1]
			if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != op.Results[1].ID || len(branch.Successors) != 2 {
				return fmt.Errorf("%s must immediately branch on failed", op.Kind)
			}
			success := branch.Successors[1].Block
			if success == block.ID || blocks[success] == nil || preds[success] != 1 {
				return fmt.Errorf("%s requires a distinct success edge", op.Kind)
			}
			candidate := op.Results[0].ID
			for _, argument := range branch.Successors[0].Arguments {
				if argument == candidate {
					return fmt.Errorf("%s candidate escapes on its failure edge", op.Kind)
				}
			}
			for _, useBlock := range fn.Blocks {
				for useIndex, use := range useBlock.Operations {
					// Transferring the candidate as a success block argument is legal.
					if useBlock.ID == block.ID && useIndex == index+1 {
						continue
					}
					for _, operand := range operationOperands(use) {
						if operand == candidate && !dom[useBlock.ID][success] {
							return fmt.Errorf("%s candidate used outside its success region", op.Kind)
						}
					}
				}
			}
		}
	}
	return nil
}

// formatArenaOperation prints semantic plan references and separate state/epoch
// transitions deterministically, without physical pointers or layout guesses.
// Rules: rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §27.
func formatArenaOperation(out *strings.Builder, op Operation) {
	for i, operand := range op.Operands {
		if i > 0 {
			out.WriteByte(',')
		}
		fmt.Fprintf(out, " %%%d", operand)
	}
	f := op.Arena
	if f == nil {
		return
	}
	formatBackingMetadata(out, op.Backing)
	formatArenaParent(out, f.Parent)
	fmt.Fprintf(out, " {state=#%d->#%d epoch=#%d->#%d backing=%s growth=%s atomic=%t", f.StateBefore, f.StateAfter, f.EpochBefore, f.EpochAfter, f.BackingKind, f.GrowthKind, f.FailureAtomic)
	fmt.Fprintf(out, " provider=%q capacity=%q layout=%q default=%q dependency=%q retained-borrow=#%d}", f.ProviderPlan, f.CapacityPlan, f.LayoutPlan, f.DefaultPlan, f.DependencyProof, f.RetainedBorrow)
	if e := f.EpochAdvance; e != (ArenaEpochAdvanceFacts{}) {
		fmt.Fprintf(out, " {epoch-advance=%s width=%d exhaustion=%s reason=%s#%d proof=%q}", e.Form, e.LogicalEpochBits, e.Exhaustion, e.PanicReason, e.PanicReasonID, e.Proof)
	}
	if op.Reference != nil {
		formatReferenceMetadata(out, op.Reference)
	}
	if op.Ownership != nil {
		fmt.Fprintf(out, " destruction=%q cause=%s", op.Ownership.DestructionPlan, op.Ownership.Cause)
	}
}

// arenaUintOperand requires source uint, not another unsigned carrier with a
// coincidentally equal width. Count is elements; capacity is bytes.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§90–93.
func arenaUintOperand(types *TypeTable, id TypeID) bool {
	t, ok := types.Lookup(id)
	return ok && t.Kind == TypeUint && t.TargetSize
}

// verifyArenaChains orders the effect sites of each Arena domain within a
// block: every operation after the creation continues the domain's owner
// state-version and epoch chain exactly (state_before and epoch_before equal
// the previous operation's state_after and epoch_after), creation
// establishes a domain once per function, and nothing follows a terminal
// Release or Destroy. Across blocks the builder's join approximation stands,
// as in --sec-verify-arenas.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 11, 36-48, 86-96, 156;
// rules/mlir/dialect-versions/sec_mlir_dialect_v15.md — § 9.
func verifyArenaChains(fn *Function) error {
	created := map[uint32]bool{}
	for _, block := range fn.Blocks {
		type step struct {
			state, epoch uint32
			terminal     bool
		}
		last := map[uint32]step{}
		for _, op := range block.Operations {
			if !isArenaOperation(op.Kind) || op.Arena == nil || op.Backing == nil || op.Backing.Domain == 0 {
				continue
			}
			domain, facts := op.Backing.Domain, op.Arena
			terminal := op.Kind == OpArenaRelease || op.Kind == OpArenaDestroy
			switch op.Kind {
			case OpArenaCreateBorrowed, OpArenaCreateOwnedFixed, OpArenaCreateGrowable, OpArenaCreateDefault:
				if created[domain] {
					return fmt.Errorf("function %s establishes Arena domain %d twice", fn.ID, domain)
				}
				created[domain] = true
			default:
				if previous, ok := last[domain]; ok {
					if previous.terminal {
						return fmt.Errorf("function %s uses Arena domain %d after its Release ended it (%s)", fn.ID, domain, op.Kind)
					}
					if facts.StateBefore != previous.state || facts.EpochBefore != previous.epoch {
						return fmt.Errorf("function %s: %s breaks the state-version or epoch chain of Arena domain %d", fn.ID, op.Kind, domain)
					}
				}
			}
			last[domain] = step{state: facts.StateAfter, epoch: facts.EpochAfter, terminal: terminal}
		}
	}
	return verifyArenaPublication(fn)
}

// verifyArenaPublication enforces failure atomicity at the IR level (P19
// §§ 35, 90-91, 151; v15 § 14): a checked creation, New or Alloc ends its
// block with the branch on its own failed flag; the candidate never crosses
// the failure edge (the success edge may pass it, §§ 90-91), and every other
// use of it is dominated by the success edge, whose block has no other
// predecessor. A failed allocation therefore never publishes a partial
// or uninitialized reference or slice.
func verifyArenaPublication(fn *Function) error {
	blocks := map[BlockID]*Block{}
	preds := map[BlockID][]BlockID{}
	for i := range fn.Blocks {
		blocks[fn.Blocks[i].ID] = fn.Blocks[i]
	}
	for _, block := range fn.Blocks {
		if n := len(block.Operations); n > 0 {
			for _, successor := range block.Operations[n-1].Successors {
				preds[successor.Block] = append(preds[successor.Block], block.ID)
			}
		}
	}
	var dom map[BlockID]map[BlockID]bool
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			if !arenaCheckedOperation(op.Kind) || len(op.Results) != 3 {
				continue
			}
			candidate, failed := op.Results[0].ID, op.Results[1].ID
			if index != len(block.Operations)-2 {
				return fmt.Errorf("function %s: %s must end its block with the branch on its failed flag", fn.ID, op.Kind)
			}
			branch := block.Operations[index+1]
			if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != failed || len(branch.Successors) != 2 {
				return fmt.Errorf("function %s: %s must branch on its failed flag before publishing", fn.ID, op.Kind)
			}
			success := branch.Successors[1].Block
			for _, argument := range branch.Successors[0].Arguments {
				if argument == candidate {
					return fmt.Errorf("function %s: %s passes its candidate across the failure branch", fn.ID, op.Kind)
				}
			}
			if len(preds[success]) != 1 {
				return fmt.Errorf("function %s: %s success edge must be the only way into its publication block", fn.ID, op.Kind)
			}
			if dom == nil {
				dom = dominators(fn.Entry, blocks, preds)
			}
			for _, user := range fn.Blocks {
				for _, use := range user.Operations {
					uses := false
					for _, operand := range use.Operands {
						uses = uses || operand == candidate
					}
					for _, edge := range use.Successors {
						for _, argument := range edge.Arguments {
							uses = uses || argument == candidate && user.ID != block.ID
						}
					}
					if uses && (user.ID == block.ID || !dom[user.ID][success]) {
						return fmt.Errorf("function %s: %s candidate is used where the allocation may have failed", fn.ID, op.Kind)
					}
				}
			}
		}
	}
	return nil
}
