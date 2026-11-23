package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildRuneArrayMaterialization lowers `text.ToRuneArray()` from Sema's
// materialization plan. A shared StringView over the receiver Place is
// formed once; a compiler-internal temporary owner starts as the empty
// rune[] and each decoded rune is appended through the ordinary P18 growth
// protocol, so the owner always holds an initialized prefix. When decoding
// ends the view ends and the owner moves into Ok. A failed growth ends the
// view, destroys the initialized prefix and ends/reclaims the backing
// through the owner's destruction plan (partial construction), and yields
// Err(CollectionError.AllocationFailed). No string bytes are copied, no
// panic endpoint exists and nothing escapes the call.
// Rules: rules/library/core-library.md — § 5.3 (ToRuneArray, MD-072);
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 25-37, 54, 72-73;
// rules/control-flow/flowcontrol_for.md — § 19 (rune decoding).
func (fb *functionBuilder) buildRuneArrayMaterialization(call *ast.CallExpression, member *ast.MemberExpression, plan sema.ResolvedRuneArrayMaterializationPlan) (builtValue, error) {
	token := member.Property.Token
	loc := location(token)
	allocation := plan.Allocation
	if fb.owner.maxPackage < dynamicArrayLoweringPackage {
		return builtValue{}, fb.unsupported("string.ToRuneArray materialization (P18)", token)
	}
	if !plan.View.Checkable() || plan.View.MutableAuthority || allocation.Allocation == 0 || allocation.Transition.Domain == 0 ||
		allocation.Transition.WhenAbsent != sema.BackingTransitionEstablishDomain || allocation.Transition.Growth != sema.BackingTransitionAdvanceEpoch {
		return builtValue{}, fb.unsupported("string.ToRuneArray without complete view, allocation and transition facts", token)
	}
	resultType, err := fb.owner.internType(plan.ResultType)
	if err != nil {
		return builtValue{}, err
	}
	arrayType, err := fb.owner.internType(allocation.ArrayType)
	if err != nil {
		return builtValue{}, err
	}
	runeType, err := fb.owner.internType(allocation.ElementType)
	if err != nil {
		return builtValue{}, err
	}
	collectionType, err := fb.owner.internType(allocation.FailureType)
	if err != nil {
		return builtValue{}, err
	}
	allocationType, err := fb.owner.internType(allocation.AllocationFailureType)
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
	destroyPlan, err := fb.owner.importDestructionPlans(sema.ResolveDestructionPlans(allocation.ArrayType), allocation.ArrayType)
	if err != nil {
		return builtValue{}, err
	}
	boolType, err := fb.boolType()
	if err != nil {
		return builtValue{}, err
	}
	uintType := fb.owner.module.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})

	// The receiver Place first, then the whole-string view (open bounds).
	sourceExpr := &ast.RefExpression{Token: token, Value: member.Object}
	source, _, err := fb.fixedSliceSourcePlace(sourceExpr, sema.ResolvedReferencePlan{Facts: plan.View, Place: plan.Source, Source: token})
	if err != nil {
		return builtValue{}, err
	}
	length := fb.result(Operation{Kind: OpStringRuneLength, Operands: []ValueID{source.id}, Location: loc}, uintType)
	start, extent := fb.newValue(uintType, OwnershipImmediate, loc), fb.newValue(uintType, OwnershipImmediate, loc)
	fb.emit(Operation{Kind: OpSliceRangeNormalize, Operands: []ValueID{length.id}, Results: []Value{start, extent},
		SliceRange: &SliceRangeMetadata{ProofKind: SliceRangeProofOpenBound}, Location: loc})
	facts := referenceMetadataOf(plan.View)
	viewType := fb.owner.module.Types.stringViewType()
	view := fb.newValue(viewType, OwnershipSharedReference, loc)
	origin := SliceFacts{Reference: *facts, RuntimeLength: extent.ID, OriginPlaces: []ValueID{source.id}}
	detached := origin.Clone()
	fb.emit(Operation{Kind: OpStringViewBorrow, Operands: []ValueID{source.id, start.ID, extent.ID}, Results: []Value{view}, Reference: facts, Slice: &detached, Location: loc})
	fb.recordReference(view.ID, facts)

	// The temporary owner: the empty rune[] in compiler storage.
	id := fb.nextStorage
	fb.nextStorage++
	fb.fn.Storages = append(fb.fn.Storages, Storage{ID: id, Name: "ToRuneArray.result", Type: arrayType, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true, Location: loc})
	fb.emit(Operation{Kind: OpStorageDeclare, Storage: id, Location: loc})
	stored := ownedStorage{storage: id, place: plan.Owner.Place, typ: arrayType, scope: fb.currentCleanupScope()}
	empty := fb.newValue(arrayType, OwnershipOwned, loc)
	fb.emit(Operation{Kind: OpDynamicArrayEmpty, Results: []Value{empty}, Location: loc})
	fb.emit(Operation{Kind: OpOwnInitializePlace, Operands: []ValueID{fb.ownedStoragePlace(stored, token).id, empty.ID}, Ownership: &OwnershipFacts{}, Location: loc})

	zero := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(0), Location: loc}, uintType)
	cursor := fb.forStorage("ToRuneArray.cursor", uintType, zero, loc)
	loop := &activeLoop{id: fb.nextLoop}
	fb.nextLoop++
	record := LoopRecord{ID: loop.id, Kind: LoopForStringView, ContinuesAfter: true, Location: loc}
	recordIndex := len(fb.fn.Loops)
	fb.fn.Loops = append(fb.fn.Loops, record)
	condition, body, done, failure, join := fb.newBlock(), fb.newBlock(), fb.newBlock(), fb.newBlock(), fb.newBlock()
	record.ConditionBlock, record.BodyBlock, record.ExitBlock = condition.ID, body.ID, done.ID
	allocationError := fb.newValue(allocationType, OwnershipImmediate, loc)
	failure.Parameters = []Value{allocationError}
	joined := fb.newValue(resultType, OwnershipImmediate, loc)
	join.Parameters = []Value{joined}
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeEnter, Location: loc})

	fb.current = condition
	position := fb.result(Operation{Kind: OpStorageLoad, Storage: cursor, Location: loc}, uintType)
	guard, err := fb.validateReference(builtValue{id: view.ID, typ: viewType}, facts, loc)
	if err != nil {
		return builtValue{}, err
	}
	has, decoded, next := fb.newValue(boolType, OwnershipImmediate, loc), fb.newValue(runeType, OwnershipImmediate, loc), fb.newValue(uintType, OwnershipImmediate, loc)
	fb.emit(Operation{Kind: OpStringViewDecodeNext, Operands: []ValueID{view.ID, position.id}, Results: []Value{has, decoded, next}, ReferenceGuard: guard, Location: loc})
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{has.ID}, Successors: []BranchTarget{{Block: body.ID}, {Block: done.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})

	fb.current = body
	fb.emit(Operation{Kind: OpStorageStore, Storage: cursor, Operands: []ValueID{next.ID}, Location: loc})
	owner := fb.ownedStoragePlace(stored, token)
	fb.emitEnsuredAppend(owner, Operation{Kind: OpDynamicArrayAppendPrepared, Operands: []ValueID{owner.id, decoded.ID}, Location: loc}, big.NewInt(1), allocation, allocationType, failure, loc)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeBack, Location: loc})

	endView := func() {
		fb.emit(Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{view.ID}, Reference: cloneReferenceMetadata(facts), Location: loc, Synthesized: true})
	}
	fb.current = done
	endView()
	finished := fb.ownedStoragePlace(stored, token)
	moved := fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{finished.id}, Ownership: &OwnershipFacts{}, Location: loc}, arrayType)
	success := fb.result(Operation{Kind: OpResultOk, Operands: []ValueID{moved.id}, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID, Arguments: []ValueID{success.id}}}, Location: loc})

	fb.current = failure
	endView()
	partial := fb.ownedStoragePlace(stored, token)
	fb.emit(Operation{Kind: OpOwnDestroyPlace, Operands: []ValueID{partial.id}, Ownership: &OwnershipFacts{Cause: "temporary-end", DestructionPlan: destroyPlan}, Location: loc})
	reason := fb.result(Operation{Kind: OpEnumConstant, EnumCase: allocationFailed, Location: loc}, collectionType)
	failed := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{reason.id}, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID, Arguments: []ValueID{failed.id}}}, Location: loc})

	fb.fn.Loops[recordIndex] = record
	fb.current = join
	if resolved, ok := fb.owner.module.Types.Lookup(resultType); !ok || resolved.Kind != TypeResult {
		return builtValue{}, fmt.Errorf("ToRuneArray result type is not a Result")
	}
	return builtValue{id: joined.ID, typ: resultType}, nil
}
