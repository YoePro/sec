package semantic

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Package 18 storage transitions, allocation backing and growth.
//
// The general storage transitions (§§ 58–62) are not dynamic-array-only:
//   - storage.establish-domain: Absent -> Live(initial epoch) with origin,
//     backing relation, reclamation authority, stability and memory space;
//   - storage.advance-epoch: Live(old) -> Live(new) for the same logical
//     domain, invalidating every dependency on the previous incarnation;
//   - storage.end-domain: Live -> Ended, distinct from reclamation;
//   - storage.reclaim: the paired physical reclamation of a resolved plan,
//     never element destruction.
//
// The dynamic-array growth operations (§§ 63–67) observe and change only
// the owner's descriptor state, never a physical capacity a program can see:
//   - dynamic-array.has-backing: whether the owner has a backing domain;
//   - dynamic-array.has-capacity: whether `count` more elements fit without
//     growth;
//   - dynamic-array.grow-backing: the fallible backing establishment or
//     growth with its static canonical transition, yielding failed and the
//     allocation-layer AllocationError; failure leaves the owner unchanged;
//   - dynamic-array.append-prepared: initializes the element at the old
//     length and increments the length; it never allocates;
//   - dynamic-array.allocate: a candidate empty owner with at least `count`
//     capacity and an established backing domain, or failed with
//     AllocationError, under which the candidate is not a valid result
//     (§§ 63–64). Its producers are the core materializations (P18-18).
const (
	OpStorageEstablishDomain     OpKind = "storage.establish-domain"
	OpStorageAdvanceEpoch        OpKind = "storage.advance-epoch"
	OpStorageEndDomain           OpKind = "storage.end-domain"
	OpStorageReclaim             OpKind = "storage.reclaim"
	OpDynamicArrayHasBacking     OpKind = "dynamic-array.has-backing"
	OpDynamicArrayHasCapacity    OpKind = "dynamic-array.has-capacity"
	OpDynamicArrayGrowBacking    OpKind = "dynamic-array.grow-backing"
	OpDynamicArrayAppendPrepared OpKind = "dynamic-array.append-prepared"
	// OpDynamicArrayAppendRangePrepared appends the count consecutive values
	// lower, lower+1, ... of a compile-time range segment into capacity ensured
	// for exactly that count; one compact operation, never one per element
	// (package18 §§ 35, 65; collections.md § 6.7; P14 compact range segments).
	OpDynamicArrayAppendRangePrepared OpKind = "dynamic-array.append-range-prepared"
	OpDynamicArrayAllocate            OpKind = "dynamic-array.allocate"
	// OpDynamicArrayClear destroys every live element in reverse logical
	// index order, sets Len to 0 and retains the backing (collections.md
	// § 6.8). It never allocates and never changes the backing epoch.
	OpDynamicArrayClear OpKind = "dynamic-array.clear"
	// OpDynamicArrayRemoveAt extracts the element at a guarded in-bounds
	// index, shifts later live elements left and decrements Len, yielding
	// the element (collections.md § 6.9). It never allocates and changes no
	// backing epoch.
	OpDynamicArrayRemoveAt OpKind = "dynamic-array.remove-at"
	// OpDynamicArrayPayloadSize observes instance SizeOf: Len times the
	// element layout stride (Integer) as a uint, without revalidating the
	// product (collections.md § 3.4).
	OpDynamicArrayPayloadSize OpKind = "dynamic-array.payload-size"
)

// BackingMetadata carries resolved Sema storage and allocation facts; it
// selects no allocator, address or layout (package18 §§ 21–31, 56, 59).
type BackingMetadata struct {
	Domain     uint32
	Transition string
	Allocation uint32
	Context    uint32
	Origin     string
	Relation   string
	Authority  string
	// Reclamation is the paired ReclamationPlanID; it exists only for
	// OwningValue authority (§ 31).
	Reclamation uint32
	Stability   string
	Space       string
}

func formatBackingMetadata(out *strings.Builder, backing *BackingMetadata) {
	if backing == nil {
		return
	}
	fmt.Fprintf(out, " {domain=#%d", backing.Domain)
	if backing.Transition != "" {
		fmt.Fprintf(out, " transition=%s", backing.Transition)
	}
	if backing.Allocation != 0 {
		fmt.Fprintf(out, " allocation=#%d context=#%d", backing.Allocation, backing.Context)
	}
	if backing.Origin != "" {
		fmt.Fprintf(out, " origin=%s relation=%s authority=%s", backing.Origin, backing.Relation, backing.Authority)
	}
	if backing.Reclamation != 0 {
		fmt.Fprintf(out, " reclamation=#%d", backing.Reclamation)
	}
	if backing.Stability != "" {
		fmt.Fprintf(out, " stability=%s space=%s", backing.Stability, backing.Space)
	}
	out.WriteByte('}')
}

func isStorageTransition(kind OpKind) bool {
	switch kind {
	case OpStorageEstablishDomain, OpStorageAdvanceEpoch, OpStorageEndDomain, OpStorageReclaim:
		return true
	}
	return false
}

func isDynamicArrayGrowth(kind OpKind) bool {
	switch kind {
	case OpDynamicArrayHasBacking, OpDynamicArrayHasCapacity, OpDynamicArrayGrowBacking, OpDynamicArrayAppendPrepared, OpDynamicArrayAppendRangePrepared, OpDynamicArrayAllocate:
		return true
	}
	return false
}

// dynamicArrayOwnerOperand checks that the first operand is a Place of a
// dynamic array, writable when the operation changes the owner.
func dynamicArrayOwnerOperand(types *TypeTable, op Operation, values map[ValueID]Value, writable bool) (Type, error) {
	if len(op.Operands) == 0 {
		return Type{}, fmt.Errorf("%s requires the owner Place", op.Kind)
	}
	place, ok := types.Lookup(values[op.Operands[0]].Type)
	if !ok || place.Kind != TypePlace {
		return Type{}, fmt.Errorf("%s addresses the owner through a Place", op.Kind)
	}
	array, ok := types.Lookup(place.Element)
	if !ok || array.Kind != TypeDynamicArray {
		return Type{}, fmt.Errorf("%s requires a Place of a dynamic array", op.Kind)
	}
	if writable && !place.Mutable {
		return Type{}, fmt.Errorf("%s changes the owner and requires a writable Place", op.Kind)
	}
	return array, nil
}

func validAuthority(authority string) bool {
	switch sema.ReclamationAuthority(authority) {
	case sema.ReclamationOwningValue, sema.ReclamationArena, sema.ReclamationPlatform, sema.ReclamationNone:
		return true
	}
	// Remaining canonical authorities are accepted by name; Unknown is not.
	return authority != "" && sema.ReclamationAuthority(authority) != sema.ReclamationUnknown
}

// verifyBackingFacts checks the complete establishment facts (§ 59) and the
// reclamation pairing (§ 31).
func verifyBackingFacts(op Operation) error {
	backing := op.Backing
	if backing.Origin == "" || backing.Relation != string(sema.BackingRelationOwned) || !validAuthority(backing.Authority) ||
		backing.Stability == "" || backing.Space == "" {
		return fmt.Errorf("%s requires origin, owned backing relation, known reclamation authority, stability and memory space", op.Kind)
	}
	if (backing.Reclamation != 0) != (backing.Authority == string(sema.ReclamationOwningValue)) {
		return fmt.Errorf("%s pairs a reclamation plan only with owning-value authority", op.Kind)
	}
	return nil
}

func isAllocationErrorType(types *TypeTable, id TypeID) bool {
	typ, ok := types.Lookup(id)
	return ok && typ.Kind == TypeEnum && typ.Identity == "core::AllocationError"
}

// verifyDynamicArrayStorageOperation checks operation shapes.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 58–67.
func verifyDynamicArrayStorageOperation(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Successors) != 0 {
		return fmt.Errorf("%s has no successors", op.Kind)
	}
	switch op.Kind {
	case OpDynamicArrayHasBacking, OpDynamicArrayHasCapacity:
		if _, err := dynamicArrayOwnerOperand(types, op, values, false); err != nil {
			return err
		}
		if len(op.Operands) != 1 || len(op.Results) != 1 || !typeHasKind(types, op.Results[0].Type, TypeBool) {
			return fmt.Errorf("%s observes one owner Place and produces bool", op.Kind)
		}
		if op.Kind == OpDynamicArrayHasCapacity && (op.Integer == nil || op.Integer.Sign() <= 0) {
			return fmt.Errorf("dynamic-array.has-capacity requires a positive element count")
		}
	case OpDynamicArrayGrowBacking:
		if _, err := dynamicArrayOwnerOperand(types, op, values, true); err != nil {
			return err
		}
		if len(op.Operands) != 1 || len(op.Results) != 2 || !typeHasKind(types, op.Results[0].Type, TypeBool) || !isAllocationErrorType(types, op.Results[1].Type) {
			return fmt.Errorf("dynamic-array.grow-backing produces failed and AllocationError")
		}
		if op.Integer == nil || op.Integer.Sign() <= 0 {
			return fmt.Errorf("dynamic-array.grow-backing requires a positive element count")
		}
		if op.Backing == nil || op.Backing.Domain == 0 || op.Backing.Allocation == 0 || op.Backing.Context == 0 {
			return fmt.Errorf("dynamic-array.grow-backing requires its backing domain, allocation identity and context")
		}
		switch sema.BackingTransitionKind(op.Backing.Transition) {
		case sema.BackingTransitionEstablishDomain, sema.BackingTransitionAdvanceEpoch, sema.BackingTransitionReplaceDomain:
		default:
			return fmt.Errorf("dynamic-array.grow-backing transition %q is not establish-domain, advance-epoch or replace-domain", op.Backing.Transition)
		}
		return verifyBackingFacts(op)
	case OpDynamicArrayAllocate:
		if len(op.Operands) != 0 || len(op.Results) != 3 {
			return fmt.Errorf("dynamic-array.allocate produces a candidate owner, failed and AllocationError")
		}
		candidate, ok := types.Lookup(op.Results[0].Type)
		if !ok || candidate.Kind != TypeDynamicArray || op.Results[0].Ownership != OwnershipOwned ||
			!typeHasKind(types, op.Results[1].Type, TypeBool) || !isAllocationErrorType(types, op.Results[2].Type) {
			return fmt.Errorf("dynamic-array.allocate produces an owned dynamic array, failed and AllocationError")
		}
		if op.Integer == nil || op.Integer.Sign() <= 0 {
			return fmt.Errorf("dynamic-array.allocate requires a positive minimum capacity")
		}
		if op.Backing == nil || op.Backing.Domain == 0 || op.Backing.Allocation == 0 || op.Backing.Context == 0 ||
			op.Backing.Transition != string(sema.BackingTransitionEstablishDomain) {
			return fmt.Errorf("dynamic-array.allocate establishes its backing domain with allocation identity and context")
		}
		return verifyBackingFacts(op)
	case OpDynamicArrayAppendPrepared:
		array, err := dynamicArrayOwnerOperand(types, op, values, true)
		if err != nil {
			return err
		}
		if len(op.Operands) != 2 || len(op.Results) != 0 || values[op.Operands[1]].Type != array.Element {
			return fmt.Errorf("dynamic-array.append-prepared transfers one element of the array's element type")
		}
	case OpDynamicArrayClear:
		if _, err := dynamicArrayOwnerOperand(types, op, values, true); err != nil {
			return err
		}
		if len(op.Operands) != 1 || len(op.Results) != 0 || op.Integer != nil || op.Backing != nil {
			return fmt.Errorf("dynamic-array.clear takes only the writable owner Place and has no result or backing transition")
		}
		if op.Ownership != nil && (op.Ownership.DestructionPlan == "" || op.Ownership.Cause != "" || op.Ownership.IfOwned) {
			return fmt.Errorf("dynamic-array.clear carries only its element destruction plan")
		}
	case OpDynamicArrayPayloadSize:
		if _, err := dynamicArrayOwnerOperand(types, op, values, false); err != nil {
			return err
		}
		result, ok := types.Lookup(op.Results[0].Type)
		if len(op.Operands) != 1 || len(op.Results) != 1 || !ok || result.Kind != TypeUint || result.Name != "uint" || !result.TargetSize ||
			op.Integer == nil || op.Integer.Sign() < 0 {
			return fmt.Errorf("dynamic-array.payload-size observes the owner Place with a non-negative stride and yields uint")
		}
	case OpDynamicArrayRemoveAt:
		array, err := dynamicArrayOwnerOperand(types, op, values, true)
		if err != nil {
			return err
		}
		if len(op.Operands) != 2 || len(op.Results) != 1 || !isBuiltinIntegerType(types, values[op.Operands[1]].Type) || op.Results[0].Type != array.Element {
			return fmt.Errorf("dynamic-array.remove-at takes the writable owner Place and an integer index and yields one element")
		}
		if op.ArrayCheckKind != ArrayIndexRuntimeCheck || op.ArrayProofKind != ArrayIndexProofGuarded || op.ArrayGuard == 0 {
			return fmt.Errorf("dynamic-array.remove-at requires its runtime index-in-bounds guard")
		}
		guard, ok := values[op.ArrayGuard]
		if !ok || !typeHasKind(types, guard.Type, TypeBool) {
			return fmt.Errorf("dynamic-array.remove-at guard is not a bool predicate")
		}
	case OpDynamicArrayAppendRangePrepared:
		array, err := dynamicArrayOwnerOperand(types, op, values, true)
		if err != nil {
			return err
		}
		if len(op.Operands) != 2 || len(op.Results) != 0 || values[op.Operands[1]].Type != array.Element {
			return fmt.Errorf("dynamic-array.append-range-prepared takes the owner and a lower bound of the array's element type")
		}
		if op.Integer == nil || op.Integer.Sign() <= 0 {
			return fmt.Errorf("dynamic-array.append-range-prepared requires a positive value count")
		}
		if element, ok := types.Lookup(array.Element); !ok || !isRangeSegmentElement(element) {
			return fmt.Errorf("dynamic-array.append-range-prepared requires a built-in integer or rune element")
		}
	case OpStorageEstablishDomain, OpStorageAdvanceEpoch, OpStorageEndDomain, OpStorageReclaim:
		if len(op.Results) != 0 || len(op.Operands) != 1 {
			return fmt.Errorf("%s applies to one owner Place and has no result", op.Kind)
		}
		if _, err := dynamicArrayOwnerOperand(types, op, values, true); err != nil {
			return err
		}
		if op.Backing == nil || op.Backing.Domain == 0 {
			return fmt.Errorf("%s requires its invalidation domain", op.Kind)
		}
		switch op.Kind {
		case OpStorageEstablishDomain:
			return verifyBackingFacts(op)
		case OpStorageReclaim:
			if op.Backing.Reclamation == 0 || op.Backing.Authority != string(sema.ReclamationOwningValue) {
				return fmt.Errorf("storage.reclaim consumes a resolved owning-value reclamation plan")
			}
		}
	}
	return nil
}

// verifyDynamicArrayTransitions checks the growth CFG (§§ 37, 64–67):
// grow-backing is immediately tested on failed; the matching storage
// transition opens its success edge, which has no other entry; an
// establishing growth follows a false has-backing test and an advancing one
// a false has-capacity test of the same owner Place; and append-prepared
// opens a block entered only from a true has-capacity test or a completed
// transition of the same owner Place.
func verifyDynamicArrayTransitions(fn *Function) error {
	type guard struct {
		owner         ValueID
		trueB, falseB BlockID
		transition    string
		domain        uint32
	}
	predecessors := map[BlockID][]BlockID{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, successor := range op.Successors {
				predecessors[successor.Block] = append(predecessors[successor.Block], block.ID)
			}
		}
	}
	blockByID := map[BlockID]*Block{}
	for _, block := range fn.Blocks {
		blockByID[block.ID] = block
	}
	tested := func(block *Block, index int, op Operation) (BlockID, BlockID, error) {
		if index+1 != len(block.Operations)-1 {
			return 0, 0, fmt.Errorf("%s in ^%d must be immediately tested", op.Kind, block.ID)
		}
		branch := block.Operations[index+1]
		if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != op.Results[0].ID || len(branch.Successors) != 2 {
			return 0, 0, fmt.Errorf("%s in ^%d must be immediately tested", op.Kind, block.ID)
		}
		return branch.Successors[0].Block, branch.Successors[1].Block, nil
	}
	backingTests, capacityTests, growths := map[BlockID]guard{}, map[BlockID]guard{}, map[BlockID]guard{}
	capacityTrue := map[BlockID][]guard{}
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			switch op.Kind {
			case OpDynamicArrayAllocate:
				if index+1 != len(block.Operations)-1 {
					return fmt.Errorf("dynamic-array.allocate in ^%d must be immediately tested", block.ID)
				}
				branch := block.Operations[index+1]
				if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != op.Results[1].ID || len(branch.Successors) != 2 {
					return fmt.Errorf("dynamic-array.allocate in ^%d must be immediately tested on failed", block.ID)
				}
				for _, argument := range branch.Successors[0].Arguments {
					if argument == op.Results[0].ID {
						return fmt.Errorf("dynamic-array.allocate candidate %%%d escapes on its failure edge", argument)
					}
				}
				if failure := blockByID[branch.Successors[0].Block]; failure != nil && len(predecessors[failure.ID]) == 1 {
					for _, use := range failure.Operations {
						for _, operand := range operationOperands(use) {
							if operand == op.Results[0].ID {
								return fmt.Errorf("dynamic-array.allocate candidate %%%d is used on its failure edge", operand)
							}
						}
					}
				}
			case OpDynamicArrayHasBacking, OpDynamicArrayHasCapacity, OpDynamicArrayGrowBacking:
				trueB, falseB, err := tested(block, index, op)
				if err != nil {
					return err
				}
				g := guard{owner: op.Operands[0], trueB: trueB, falseB: falseB}
				switch op.Kind {
				case OpDynamicArrayHasBacking:
					backingTests[falseB] = g
				case OpDynamicArrayHasCapacity:
					capacityTests[falseB] = g
					capacityTrue[trueB] = append(capacityTrue[trueB], g)
				default:
					g.transition, g.domain = op.Backing.Transition, op.Backing.Domain
					growths[falseB] = g
					if index != 0 {
						return fmt.Errorf("dynamic-array.grow-backing in ^%d must open its block after its state test", block.ID)
					}
					var entry map[BlockID]guard
					switch sema.BackingTransitionKind(g.transition) {
					case sema.BackingTransitionEstablishDomain:
						entry = backingTests
					default:
						entry = capacityTests
					}
					test, found := entry[block.ID]
					if !found || test.owner != g.owner || len(predecessors[block.ID]) != 1 {
						return fmt.Errorf("dynamic-array.grow-backing %s in ^%d must follow the failed state test of the same owner", g.transition, block.ID)
					}
				}
			}
		}
	}
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			switch op.Kind {
			case OpStorageEstablishDomain, OpStorageAdvanceEpoch:
				g, found := growths[block.ID]
				want := sema.BackingTransitionEstablishDomain
				if op.Kind == OpStorageAdvanceEpoch {
					want = sema.BackingTransitionAdvanceEpoch
				}
				if index != 0 || !found || g.transition != string(want) || g.domain != op.Backing.Domain || g.owner != op.Operands[0] || len(predecessors[block.ID]) != 1 {
					return fmt.Errorf("%s in ^%d must open the success edge of the matching dynamic-array.grow-backing", op.Kind, block.ID)
				}
			case OpDynamicArrayAppendPrepared, OpDynamicArrayAppendRangePrepared:
				if index != 0 || len(predecessors[block.ID]) == 0 {
					return fmt.Errorf("dynamic-array.append-prepared in ^%d must open a block entered after capacity is ensured", block.ID)
				}
				for _, predecessor := range predecessors[block.ID] {
					ensured := false
					for _, g := range capacityTrue[block.ID] {
						ensured = ensured || g.owner == op.Operands[0] && blockEndsWithTestOf(blockByID[predecessor], g.owner)
					}
					if source := blockByID[predecessor]; !ensured && source != nil && len(source.Operations) >= 2 {
						first := source.Operations[0]
						last := source.Operations[len(source.Operations)-1]
						ensured = (first.Kind == OpStorageEstablishDomain || first.Kind == OpStorageAdvanceEpoch) && first.Operands[0] == op.Operands[0] && last.Kind == OpBranch
					}
					if !ensured {
						return fmt.Errorf("dynamic-array.append-prepared in ^%d is reachable from ^%d without ensured capacity", block.ID, predecessor)
					}
					// The ensured capacity covers exactly the appended count.
					if want := appendedCount(op); !ensuredCountMatches(blockByID, predecessors, predecessor, op.Operands[0], want) {
						return fmt.Errorf("%s in ^%d appends %s values but the capacity ensured on the edge from ^%d covers another count", op.Kind, block.ID, want.String(), predecessor)
					}
				}
			}
		}
	}
	return nil
}

// blockEndsWithTestOf reports whether a block's state test is a
// has-capacity of the owner Place.
func blockEndsWithTestOf(block *Block, owner ValueID) bool {
	if block == nil || len(block.Operations) < 2 {
		return false
	}
	test := block.Operations[len(block.Operations)-2]
	return test.Kind == OpDynamicArrayHasCapacity && test.Operands[0] == owner
}

// buildDynamicArrayAppend lowers `values.Append(value)` for a trivially
// copyable and destructible element, and `values.Append(lower..upper)` as one
// all-or-nothing growth by the segment's count followed by one compact
// append-range-prepared (an empty segment appends nothing). The receiver Place is formed first and
// the argument evaluated second. An owner without backing establishes its
// domain; an owner without spare capacity grows and advances its epoch
// (Sema rejects a live element or slice borrow, S1162); then the element is
// appended. Failure leaves the owner unchanged and yields
// CollectionError.AllocationFailed; no panic endpoint exists.
// Rules: rules/collections/collections.md — §§ 6.4, 6.7, 16;
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 25–31, 35–37, 58–68.
func (fb *functionBuilder) buildDynamicArrayAppend(call *ast.CallExpression, member *ast.MemberExpression, plan sema.ResolvedAllocationOperationPlan) (builtValue, error) {
	token := member.Property.Token
	rangeEntry, isRange := fb.owner.analyzer.ResolvedRangeAppendOf(call)
	if len(call.Arguments) != 1 || (plan.Operation != sema.AllocationDynamicArrayAppend && plan.Operation != sema.AllocationDynamicArrayAppendRange) ||
		(plan.Operation == sema.AllocationDynamicArrayAppendRange) != isRange {
		return builtValue{}, fb.unsupported("dynamic-array Append without a resolved single-value or range plan", token)
	}
	count := big.NewInt(1)
	if isRange {
		if rangeEntry.Length == nil || rangeEntry.RangeLower == nil || plan.Count == nil || plan.Count.Cmp(rangeEntry.Length) != 0 {
			return builtValue{}, fmt.Errorf("range Append plan disagrees with its resolved range segment")
		}
		count = new(big.Int).Set(rangeEntry.Length)
		if count.Sign() == 0 {
			// collections.md § 6.7: an empty segment appends nothing,
			// establishes no backing and succeeds (package18 § 22).
			return fb.buildEmptyRangeAppend(call, member)
		}
	}
	if !fb.owner.triviallyDestructible(plan.ElementType) || fb.owner.copyClassificationOf(plan.ElementType) != sema.CopyTrivial {
		return builtValue{}, fb.unsupported("Append of a non-trivial element (element ownership on failure, MD-135)", token)
	}
	if plan.Receiver == nil || plan.Allocation == 0 || plan.Transition.Domain == 0 ||
		plan.Transition.WhenAbsent != sema.BackingTransitionEstablishDomain || plan.Transition.WithinCapacity != sema.BackingTransitionNone ||
		plan.Transition.Growth != sema.BackingTransitionAdvanceEpoch || plan.Transition.OnFailure != sema.BackingTransitionNone {
		return builtValue{}, fb.unsupported("dynamic-array Append without complete resolved allocation and transition facts", token)
	}
	owner, err := fb.dynamicArrayOwnerPlace(member.Object)
	if err != nil {
		return builtValue{}, err
	}
	if typ, _ := fb.owner.module.Types.Lookup(owner.typ); !typ.Mutable {
		return builtValue{}, fmt.Errorf("dynamic-array Append on a read-only owner Place")
	}
	element, err := fb.owner.internType(plan.ElementType)
	if err != nil {
		return builtValue{}, err
	}
	var value builtValue
	if isRange {
		if typ, _ := fb.owner.module.Types.Lookup(element); !isRangeSegmentElement(typ) {
			return builtValue{}, fb.unsupported("range Append of a non-integer element", token)
		}
		// One compact segment: the constant lower bound and the value count.
		value = fb.result(Operation{Kind: OpConstInt, Integer: new(big.Int).Set(rangeEntry.RangeLower), Location: location(expressionToken(call.Arguments[0]))}, element)
	} else if value, err = fb.buildExpr(call.Arguments[0], element); err != nil {
		return builtValue{}, err
	}
	resultSema, ok := fb.owner.analyzer.TypeOf(call)
	if !ok {
		return builtValue{}, fmt.Errorf("Append has no resolved result type")
	}
	resultType, err := fb.owner.internType(resultSema)
	if err != nil {
		return builtValue{}, err
	}
	collectionType, err := fb.owner.internType(plan.FailureType)
	if err != nil {
		return builtValue{}, err
	}
	allocationType, err := fb.owner.internType(plan.AllocationFailureType)
	if err != nil {
		return builtValue{}, err
	}
	collection, ok := fb.owner.enumDefinition(collectionType)
	if !ok {
		return builtValue{}, fb.unsupported("CollectionError enum definition", token)
	}
	allocationFailed, ok := enumCaseIDByName(collection, "AllocationFailed")
	if !ok {
		return builtValue{}, fb.unsupported("CollectionError.AllocationFailed", token)
	}
	loc := location(token)
	failure, join := fb.newBlock(), fb.newBlock()
	allocationError := fb.newValue(allocationType, OwnershipImmediate, loc)
	failure.Parameters = []Value{allocationError}
	joined := fb.newValue(resultType, OwnershipImmediate, loc)
	join.Parameters = []Value{joined}
	appendOp := Operation{Kind: OpDynamicArrayAppendPrepared, Operands: []ValueID{owner.id, value.id}, Location: loc}
	if isRange {
		appendOp = Operation{Kind: OpDynamicArrayAppendRangePrepared, Operands: []ValueID{owner.id, value.id}, Integer: new(big.Int).Set(count), Location: loc}
	}
	fb.emitEnsuredAppend(owner, appendOp, count, plan, allocationType, failure, loc)
	success := fb.result(Operation{Kind: OpResultOk, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID, Arguments: []ValueID{success.id}}}, Location: loc})

	fb.current = failure
	reason := fb.result(Operation{Kind: OpEnumConstant, EnumCase: allocationFailed, Location: loc}, collectionType)
	errorResult := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{reason.id}, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID, Arguments: []ValueID{errorResult.id}}}, Location: loc})

	fb.current = join
	return builtValue{id: joined.ID, typ: resultType}, nil
}

// appendedCount is the number of values an append operation adds.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v14.md — § 13A.
func appendedCount(op Operation) *big.Int {
	if op.Kind == OpDynamicArrayAppendRangePrepared && op.Integer != nil {
		return op.Integer
	}
	return big.NewInt(1)
}

// ensuredCountMatches checks the count of the state test or growth that
// ensured capacity on the edge from predecessor: a true has-capacity test
// ending predecessor, or the grow-backing opening the single predecessor of
// a storage-transition block (package18 §§ 36–37, 65–67).
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 35–37, 64–67.
func ensuredCountMatches(blockByID map[BlockID]*Block, predecessors map[BlockID][]BlockID, predecessor BlockID, owner ValueID, want *big.Int) bool {
	source := blockByID[predecessor]
	if source == nil {
		return false
	}
	if blockEndsWithTestOf(source, owner) {
		test := source.Operations[len(source.Operations)-2]
		return test.Integer != nil && test.Integer.Cmp(want) == 0
	}
	if len(predecessors[predecessor]) != 1 {
		return false
	}
	grow := blockByID[predecessors[predecessor][0]]
	if grow == nil || len(grow.Operations) == 0 || grow.Operations[0].Kind != OpDynamicArrayGrowBacking || grow.Operations[0].Integer == nil {
		return false
	}
	return grow.Operations[0].Integer.Cmp(want) == 0
}

// isRangeSegmentElement admits the element types of a compile-time range
// segment: built-in integers, byte and rune (range_segments.go in Sema).
// Rules: rules/collections/collections.md — §§ 5.6a, 6.7.
func isRangeSegmentElement(t Type) bool {
	switch t.Kind {
	case TypeInt, TypeUint, TypeByte, TypeRune:
		return true
	}
	return false
}

// buildEmptyRangeAppend lowers Append of an empty range segment: the owner
// is unchanged, no backing is established and the result is Ok.
// Rules: rules/collections/collections.md — § 6.7; rules/mlir/packages/sec-mlir-dialect_package18.md — § 22.
func (fb *functionBuilder) buildEmptyRangeAppend(call *ast.CallExpression, member *ast.MemberExpression) (builtValue, error) {
	if _, err := fb.dynamicArrayOwnerPlace(member.Object); err != nil {
		return builtValue{}, err
	}
	resultSema, ok := fb.owner.analyzer.TypeOf(call)
	if !ok {
		return builtValue{}, fmt.Errorf("Append has no resolved result type")
	}
	resultType, err := fb.owner.internType(resultSema)
	if err != nil {
		return builtValue{}, err
	}
	return fb.result(Operation{Kind: OpResultOk, Location: location(member.Property.Token)}, resultType), nil
}

// emitEnsuredAppend emits the growth protocol of one append and the append
// itself: an owner without backing grows with establish-domain, one without
// capacity for count values grows with advance-epoch (reclaiming relocated
// backing under its paired plan), and both continue to appendOp. A failed
// growth branches to failure with its AllocationError; the owner is
// unchanged. fb.current is the block after appendOp.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 25–31, 35–37, 58–68.
func (fb *functionBuilder) emitEnsuredAppend(owner builtValue, appendOp Operation, count *big.Int, plan sema.ResolvedAllocationOperationPlan, allocationType TypeID, failure *Block, loc Location) {
	boolean := fb.owner.module.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
	facts := BackingMetadata{
		Domain: uint32(plan.Transition.Domain), Allocation: uint32(plan.Allocation), Context: uint32(plan.ContextIdentity),
		Origin: string(plan.StorageOrigin), Relation: string(plan.BackingRelation), Authority: string(plan.ReclamationAuthority),
		Reclamation: uint32(plan.ReclamationPlan), Stability: string(plan.AddressStability), Space: strings.TrimPrefix(plan.MemorySpace.String(), "space:"),
	}
	with := func(transition sema.BackingTransitionKind) *BackingMetadata {
		copied := facts
		copied.Transition = string(transition)
		return &copied
	}
	absent, existing, appendBlock := fb.newBlock(), fb.newBlock(), fb.newBlock()
	has := fb.result(Operation{Kind: OpDynamicArrayHasBacking, Operands: []ValueID{owner.id}, Location: loc}, boolean)
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{has.id}, Successors: []BranchTarget{{Block: existing.ID}, {Block: absent.ID}}, Location: loc})
	grow := func(transition sema.BackingTransitionKind, apply OpKind) {
		failed, reason := fb.newValue(boolean, OwnershipImmediate, loc), fb.newValue(allocationType, OwnershipImmediate, loc)
		fb.emit(Operation{Kind: OpDynamicArrayGrowBacking, Operands: []ValueID{owner.id}, Results: []Value{failed, reason}, Integer: new(big.Int).Set(count), Backing: with(transition), Location: loc})
		success := fb.newBlock()
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{failed.ID}, Successors: []BranchTarget{{Block: failure.ID, Arguments: []ValueID{reason.ID}}, {Block: success.ID}}, Location: loc})
		fb.current = success
		fb.emit(Operation{Kind: apply, Operands: []ValueID{owner.id}, Backing: with(transition), Location: loc})
		if apply == OpStorageAdvanceEpoch && plan.Transition.ReclaimsRelocatedBacking {
			// Relocated backing is reclaimed only under its paired plan (§§ 30–31).
			fb.emit(Operation{Kind: OpStorageReclaim, Operands: []ValueID{owner.id}, Backing: with(sema.BackingTransitionReclaim), Location: loc})
		}
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: appendBlock.ID}}, Location: loc})
	}
	fb.current = absent
	grow(sema.BackingTransitionEstablishDomain, OpStorageEstablishDomain)
	fb.current = existing
	spare := fb.result(Operation{Kind: OpDynamicArrayHasCapacity, Operands: []ValueID{owner.id}, Integer: new(big.Int).Set(count), Location: loc}, boolean)
	growBlock := fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{spare.id}, Successors: []BranchTarget{{Block: appendBlock.ID}, {Block: growBlock.ID}}, Location: loc})
	fb.current = growBlock
	grow(sema.BackingTransitionAdvanceEpoch, OpStorageAdvanceEpoch)
	fb.current = appendBlock
	fb.emit(appendOp)
}

// buildDynamicArrayClear lowers `values.Clear()` on an owning dynamic array
// to one dynamic-array.clear on the writable owner Place: it destroys every
// live element in reverse logical index order, sets Len to 0 and retains the
// backing. An element that needs destruction names its type-level
// destruction plan (the same element plan the array's own destroy-elements-
// reverse step uses); a trivially destructible element carries none, so the
// reverse destruction is vacuous. Clear never allocates, never fails and
// does not change the backing epoch.
// Rules: rules/collections/collections.md — § 6.8;
// rules/memory/destruction.md — reverse element destruction.
func (fb *functionBuilder) buildDynamicArrayClear(call *ast.CallExpression, member *ast.MemberExpression) (builtValue, error) {
	token := member.Property.Token
	if len(call.Arguments) != 0 {
		return builtValue{}, fmt.Errorf("Clear takes no arguments")
	}
	receiver, ok := fb.owner.analyzer.TypeOf(member.Object)
	if !ok {
		return builtValue{}, fmt.Errorf("Clear receiver has no resolved type")
	}
	if receiver.Kind == sema.ReferenceType && receiver.Element != nil {
		receiver = *receiver.Element
	}
	array, dynamic := sema.ResolveDynamicArrayPlan(receiver)
	if !dynamic {
		return builtValue{}, fb.unsupported("Clear on a receiver without a dynamic-array plan", token)
	}
	owner, err := fb.dynamicArrayOwnerPlace(member.Object)
	if err != nil {
		return builtValue{}, err
	}
	if typ, _ := fb.owner.module.Types.Lookup(owner.typ); !typ.Mutable {
		return builtValue{}, fmt.Errorf("dynamic-array Clear on a read-only owner Place")
	}
	op := Operation{Kind: OpDynamicArrayClear, Operands: []ValueID{owner.id}, Location: location(token)}
	if array.ElementRequiresDestruction {
		planID, err := fb.owner.importDestructionPlans(sema.ResolveDestructionPlans(array.ElementType), array.ElementType)
		if err != nil {
			return builtValue{}, fb.unsupported("Clear element destruction plan: "+err.Error(), token)
		}
		op.Ownership = &OwnershipFacts{DestructionPlan: planID}
	}
	void, err := fb.owner.internType(sema.Type{Name: "void", Kind: sema.VoidType})
	if err != nil {
		return builtValue{}, err
	}
	fb.emit(op)
	return builtValue{typ: void}, nil
}

// verifyClearElementPlan checks that a Clear names exactly the module plan of
// its element type when that plan destroys anything, and no plan otherwise.
// Rules: rules/collections/collections.md — § 6.8.
func verifyClearElementPlan(module *Module, op Operation, values map[ValueID]Value) error {
	place, _ := module.Types.Lookup(values[op.Operands[0]].Type)
	array, _ := module.Types.Lookup(place.Element)
	if op.Ownership != nil {
		plan, err := destructionPlan(module, op.Ownership.DestructionPlan, array.Element)
		if err != nil {
			return fmt.Errorf("dynamic-array.clear element plan: %w", err)
		}
		if plan.Kind == "trivial" {
			return fmt.Errorf("dynamic-array.clear names trivial element plan %q", plan.ID)
		}
		return nil
	}
	for _, plan := range module.DestructionPlans {
		if plan.Type == array.Element && plan.Kind != "trivial" {
			return fmt.Errorf("dynamic-array.clear of elements that need destruction lacks their plan %q", plan.ID)
		}
	}
	return nil
}

// dynamicArrayMutatorCall reports a call of the compiler-known owning
// dynamic-array Clear or RemoveAt once P18 lowering is enabled.
// Rules: rules/collections/collections.md — §§ 6.8, 6.9.
func (fb *functionBuilder) dynamicArrayMutatorCall(call *ast.CallExpression) (*ast.MemberExpression, string, bool) {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil || fb.owner.maxPackage < dynamicArrayLoweringPackage {
		return nil, "", false
	}
	site := member.Property.Token
	known, ok := fb.owner.analyzer.CompilerKnownMemberAt(site.File, site.Line, site.Column)
	if !ok || known.ID != "CKM-DYNAMIC-ARRAY-CLEAR" && known.ID != "CKM-DYNAMIC-ARRAY-REMOVEAT" {
		return nil, "", false
	}
	return member, known.ID, true
}

// buildDynamicArrayRemoveAt lowers `values.RemoveAt(index)` for a trivially
// copyable element: the total index-in-bounds predicate splits the path; the
// in-bounds edge extracts the element with dynamic-array.remove-at (later
// elements shift left, Len decrements, the former final slot becomes
// uninitialized) and builds Some(element); the other edge builds None without
// mutation. No allocation, panic or epoch change exists. An element that is
// not trivially copyable and destructible is extracted as an owned value the
// Option constructs directly; the shifted suffix stays owned by the array
// (elements relocate within the backing, never copy).
// Rules: rules/collections/collections.md — § 6.9;
// rules/mlir/packages/sec-mlir-dialect_package18.md — § 15.
func (fb *functionBuilder) buildDynamicArrayRemoveAt(call *ast.CallExpression, member *ast.MemberExpression) (builtValue, error) {
	token := member.Property.Token
	if len(call.Arguments) != 1 {
		return builtValue{}, fmt.Errorf("RemoveAt takes one index")
	}
	receiver, ok := fb.owner.analyzer.TypeOf(member.Object)
	if !ok {
		return builtValue{}, fmt.Errorf("RemoveAt receiver has no resolved type")
	}
	if receiver.Kind == sema.ReferenceType && receiver.Element != nil {
		receiver = *receiver.Element
	}
	array, dynamic := sema.ResolveDynamicArrayPlan(receiver)
	if !dynamic {
		return builtValue{}, fb.unsupported("RemoveAt on a receiver without a dynamic-array plan", token)
	}
	if array.ElementCopyClass == sema.CopyConditional {
		return builtValue{}, fb.unsupported("RemoveAt of an element with unresolved ownership", token)
	}
	// A trivially copyable, trivially destructible element is an immediate
	// value; any other element leaves the array as an owned value that the
	// Option then owns (owning extraction, § 6.9).
	owned := array.ElementCopyClass != sema.CopyTrivial || array.ElementRequiresDestruction
	indexSema, ok := fb.owner.analyzer.TypeOf(call.Arguments[0])
	if !ok || indexSema.Kind != sema.IntType && indexSema.Kind != sema.UintType {
		return builtValue{}, fb.unsupported("RemoveAt with a non-built-in integer index", token)
	}
	resultSema, ok := fb.owner.analyzer.TypeOf(call)
	if !ok {
		return builtValue{}, fmt.Errorf("RemoveAt has no resolved result type")
	}
	resultType, err := fb.owner.internType(resultSema)
	if err != nil {
		return builtValue{}, err
	}
	some, none, ok := fb.optionVariants(resultType)
	if !ok {
		return builtValue{}, fb.unsupported("RemoveAt result without its Option definition", token)
	}
	owner, err := fb.dynamicArrayOwnerPlace(member.Object)
	if err != nil {
		return builtValue{}, err
	}
	if typ, _ := fb.owner.module.Types.Lookup(owner.typ); !typ.Mutable {
		return builtValue{}, fmt.Errorf("dynamic-array RemoveAt on a read-only owner Place")
	}
	element, err := fb.owner.internType(array.ElementType)
	if err != nil {
		return builtValue{}, err
	}
	indexType, err := fb.owner.internType(indexSema)
	if err != nil {
		return builtValue{}, err
	}
	index, err := fb.buildExpr(call.Arguments[0], indexType)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(token)
	signed := indexSema.Kind == sema.IntType
	boolean := fb.owner.module.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
	predicate := fb.result(Operation{Kind: OpDynamicArrayIndexInBounds, Operands: []ValueID{owner.id, index.id}, ArrayIndexSigned: signed, Location: loc}, boolean)
	present, absent, join := fb.newBlock(), fb.newBlock(), fb.newBlock()
	ownership, action := OwnershipImmediate, UnionPayloadCopyTrivial
	if owned {
		ownership, action = OwnershipOwned, UnionPayloadConstructDirect
	}
	joined := fb.newValue(resultType, ownership, loc)
	join.Parameters = []Value{joined}
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{predicate.id}, Successors: []BranchTarget{{Block: present.ID}, {Block: absent.ID}}, Location: loc})

	fb.current = present
	removed := fb.newValue(element, ownership, loc)
	fb.emit(Operation{
		Kind: OpDynamicArrayRemoveAt, Operands: []ValueID{owner.id, index.id}, Results: []Value{removed}, ArrayIndexSigned: signed,
		ArrayCheckKind: ArrayIndexRuntimeCheck, ArrayProofKind: ArrayIndexProofGuarded, ArrayGuard: predicate.id, Location: loc,
	})
	wrapped := fb.result(Operation{Kind: OpUnionConstruct, UnionVariant: some, Operands: []ValueID{removed.ID}, PayloadActions: []UnionPayloadAction{action}, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID, Arguments: []ValueID{wrapped.id}}}, Location: loc})

	fb.current = absent
	empty := fb.result(Operation{Kind: OpUnionConstruct, UnionVariant: none, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID, Arguments: []ValueID{empty.id}}}, Location: loc})

	fb.current = join
	return builtValue{id: joined.ID, typ: resultType}, nil
}

// buildDynamicArrayPayloadSize lowers instance `values.SizeOf` on an owning
// dynamic array: the represented payload bytes Len times the element's
// canonical layout stride under the target plan. The product is not
// revalidated: growth already established a representable payload extent.
// An element without complete layout stays a named boundary.
// Rules: rules/collections/collections.md — § 3.4; rules/memory/layout.md — § 9.
func (fb *functionBuilder) buildDynamicArrayPayloadSize(member *ast.MemberExpression) (builtValue, error) {
	token := member.Property.Token
	receiver, ok := fb.owner.analyzer.TypeOf(member.Object)
	if !ok {
		return builtValue{}, fmt.Errorf("SizeOf receiver has no resolved type")
	}
	if receiver.Kind == sema.ReferenceType && receiver.Element != nil {
		receiver = *receiver.Element
	}
	array, dynamic := sema.ResolveDynamicArrayPlan(receiver)
	if !dynamic {
		return builtValue{}, fb.unsupported("SizeOf on a receiver without a dynamic-array plan", token)
	}
	stride, ok := fb.owner.analyzer.ResolvedElementStrideOf(array.ElementType)
	if !ok {
		return builtValue{}, fb.unsupported("dynamic-array SizeOf of an element without complete layout (P24)", token)
	}
	owner, err := fb.dynamicArrayOwnerPlace(member.Object)
	if err != nil {
		return builtValue{}, err
	}
	uintType, err := fb.owner.internType(sema.Type{Name: "uint", Kind: sema.UintType})
	if err != nil {
		return builtValue{}, err
	}
	return fb.result(Operation{Kind: OpDynamicArrayPayloadSize, Operands: []ValueID{owner.id}, Integer: stride, Location: location(token)}, uintType), nil
}

// buildStaticSizeOf lowers `T.SizeOf` and value-form SizeOf of a statically
// sized value to the canonical layout size of the type under the target plan,
// a uint constant. The value form keeps its receiver-expression effects: a
// receiver other than a plain name is evaluated and, being trivially
// destructible, dropped. Dynamic arrays, strings and slices have their own
// payload observations and are not handled here; a type without complete
// layout is left to the remaining paths.
// Rules: rules/compiler/compiler_known_members.md — §§ 14(4), 15(4), 16(4);
// rules/memory/layout.md — § 7.
func (fb *functionBuilder) buildStaticSizeOf(member *ast.MemberExpression, typeForm bool) (builtValue, bool, error) {
	var target sema.Type
	var ok bool
	if typeForm {
		target, ok = fb.owner.analyzer.SizeOfTypeSubject(member)
	} else {
		target, ok = fb.owner.analyzer.TypeOf(member.Object)
	}
	if !ok {
		return builtValue{}, false, nil
	}
	switch target.Kind {
	case sema.StringType, sema.StringViewType, sema.SliceType, sema.ReferenceType:
		return builtValue{}, false, nil
	}
	if _, dynamic := sema.ResolveDynamicArrayPlan(target); dynamic {
		return builtValue{}, false, nil
	}
	size, sized := fb.owner.analyzer.ResolvedTypeSizeOf(target)
	if !sized {
		return builtValue{}, false, nil
	}
	if !typeForm {
		if _, name := member.Object.(*ast.Identifier); !name {
			if !fb.owner.triviallyDestructible(target) {
				return builtValue{}, true, fb.unsupported("value-form SizeOf of a non-trivially destructible temporary", member.Property.Token)
			}
			if _, err := fb.buildExpr(member.Object, 0); err != nil {
				return builtValue{}, true, err
			}
		}
	}
	uintType, err := fb.owner.internType(sema.Type{Name: "uint", Kind: sema.UintType})
	if err != nil {
		return builtValue{}, true, err
	}
	return fb.result(Operation{Kind: OpConstInt, Integer: size, Location: location(member.Property.Token)}, uintType), true, nil
}
