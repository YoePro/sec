package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildDynamicArrayFill lowers contextual `try fill(value, count)` into an
// owning dynamic array from Sema's fill plan. The value is evaluated once,
// then the count. A compiler-internal temporary owner starts as the empty
// array and each copy is appended through the ordinary P18 growth protocol,
// so the owner always holds an initialized prefix. When count copies are
// appended the owner moves into Ok. A failed growth destroys the initialized
// prefix and ends/reclaims the backing through the owner's destruction plan
// (partial construction) and yields Err(CollectionError.AllocationFailed). No
// panic endpoint exists and nothing escapes the call. The count must be
// unsigned or a non-negative integer literal: the meaning of a negative
// signed runtime count is undecided (MD-155).
// Rules: rules/compiler/compiler_known_members.md — § 50(2);
// rules/collections/collections.md — § 17;
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 25-37.
func (fb *functionBuilder) buildDynamicArrayFill(call *ast.CallExpression, plan sema.ResolvedDynamicArrayFillPlan) (builtValue, error) {
	token := call.Token
	loc := location(token)
	allocation := plan.Allocation
	if fb.owner.maxPackage < dynamicArrayLoweringPackage {
		return builtValue{}, fb.unsupported("dynamic-array fill (P18)", token)
	}
	if len(call.Arguments) != 2 || allocation.Allocation == 0 || allocation.Transition.Domain == 0 ||
		allocation.Transition.WhenAbsent != sema.BackingTransitionEstablishDomain || allocation.Transition.Growth != sema.BackingTransitionAdvanceEpoch {
		return builtValue{}, fb.unsupported("dynamic-array fill without complete allocation and transition facts", token)
	}
	if !fb.owner.triviallyDestructible(allocation.ElementType) || fb.owner.copyClassificationOf(allocation.ElementType) != sema.CopyTrivial {
		return builtValue{}, fb.unsupported("fill of a non-trivially copyable element", token)
	}
	resultType, err := fb.owner.internType(plan.ResultType)
	if err != nil {
		return builtValue{}, err
	}
	arrayType, err := fb.owner.internType(allocation.ArrayType)
	if err != nil {
		return builtValue{}, err
	}
	element, err := fb.owner.internType(allocation.ElementType)
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

	value, err := fb.buildExpr(call.Arguments[0], element)
	if err != nil {
		return builtValue{}, err
	}
	countSema, ok := fb.owner.analyzer.TypeOf(call.Arguments[1])
	if !ok {
		return builtValue{}, fmt.Errorf("fill count has no resolved type")
	}
	var count builtValue
	var countType TypeID
	switch {
	case countSema.Kind == sema.UintType:
		if countType, err = fb.owner.internType(countSema); err != nil {
			return builtValue{}, err
		}
		if count, err = fb.buildExpr(call.Arguments[1], countType); err != nil {
			return builtValue{}, err
		}
	default:
		literal, isLiteral := call.Arguments[1].(*ast.IntegerLiteral)
		if !isLiteral || literal.Value < 0 || literal.BigValue != nil && literal.BigValue.Sign() < 0 {
			return builtValue{}, fb.unsupported("fill with a signed runtime count (negative-count semantics undecided, MD-155)", token)
		}
		countType = fb.owner.module.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
		constant := big.NewInt(literal.Value)
		if literal.BigValue != nil {
			constant = new(big.Int).Set(literal.BigValue)
		}
		count = fb.result(Operation{Kind: OpConstInt, Integer: constant, Location: location(literal.Token)}, countType)
	}

	// The temporary owner: the empty array in compiler storage.
	id := fb.nextStorage
	fb.nextStorage++
	fb.fn.Storages = append(fb.fn.Storages, Storage{ID: id, Name: "fill.result", Type: arrayType, Mutable: true, Class: StorageLocalAutomatic, AddressTaken: true, Location: loc})
	fb.emit(Operation{Kind: OpStorageDeclare, Storage: id, Location: loc})
	stored := ownedStorage{storage: id, place: plan.Owner.Place, typ: arrayType, scope: fb.currentCleanupScope()}
	empty := fb.newValue(arrayType, OwnershipOwned, loc)
	fb.emit(Operation{Kind: OpDynamicArrayEmpty, Results: []Value{empty}, Location: loc})
	fb.emit(Operation{Kind: OpOwnInitializePlace, Operands: []ValueID{fb.ownedStoragePlace(stored, token).id, empty.ID}, Ownership: &OwnershipFacts{}, Location: loc})

	zero := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(0), Location: loc}, countType)
	one := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(1), Location: loc}, countType)
	cursor := fb.forStorage("fill.cursor", countType, zero, loc)
	loop := &activeLoop{id: fb.nextLoop}
	fb.nextLoop++
	record := LoopRecord{ID: loop.id, Kind: LoopForRange, ContinuesAfter: true, Location: loc}
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
	position := fb.result(Operation{Kind: OpStorageLoad, Storage: cursor, Location: loc}, countType)
	more := fb.forCompare(IntegerCompareLT, position, count, boolType, loc)
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{more.id}, Successors: []BranchTarget{{Block: body.ID}, {Block: done.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})

	fb.current = body
	owner := fb.ownedStoragePlace(stored, token)
	fb.emitEnsuredAppend(owner, Operation{Kind: OpDynamicArrayAppendPrepared, Operands: []ValueID{owner.id, value.id}, Location: loc}, big.NewInt(1), allocation, allocationType, failure, loc)
	next, err := fb.forAdd(position, one, countType, loc)
	if err != nil {
		return builtValue{}, err
	}
	fb.emit(Operation{Kind: OpStorageStore, Storage: cursor, Operands: []ValueID{next.id}, Location: loc})
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeBack, Location: loc})

	fb.current = done
	finished := fb.ownedStoragePlace(stored, token)
	moved := fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{finished.id}, Ownership: &OwnershipFacts{}, Location: loc}, arrayType)
	success := fb.result(Operation{Kind: OpResultOk, Operands: []ValueID{moved.id}, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID, Arguments: []ValueID{success.id}}}, Location: loc})

	fb.current = failure
	partial := fb.ownedStoragePlace(stored, token)
	fb.emit(Operation{Kind: OpOwnDestroyPlace, Operands: []ValueID{partial.id}, Ownership: &OwnershipFacts{Cause: "temporary-end", DestructionPlan: destroyPlan}, Location: loc})
	reason := fb.result(Operation{Kind: OpEnumConstant, EnumCase: allocationFailed, Location: loc}, collectionType)
	failed := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{reason.id}, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID, Arguments: []ValueID{failed.id}}}, Location: loc})

	fb.fn.Loops[recordIndex] = record
	fb.current = join
	return builtValue{id: joined.ID, typ: resultType}, nil
}
