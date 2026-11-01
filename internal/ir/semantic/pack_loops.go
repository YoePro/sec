package semantic

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// LoopForPack is a `for` over a heterogeneous type pack in one
// specialization (semantic_ir.md § 27A; flowcontrol_for.md § 13A).
const LoopForPack LoopKind = "for-pack"

// buildPackFor lowers a `for` over the heterogeneous pack of the
// specialization being built. The template body was checked once with the
// opaque element `Types[value]`; here every position k gets its own copy of the
// body built with `Types[value]` bound to the concrete element k and the loop
// binding bound to position k's parameter. The positions run through an
// ordinary loop CFG: a compiler-generated position cursor, the condition
// cursor < N, and a dispatch on the cursor to the body of each position, so
// `break`, `continue`, `return` and evaluation order keep their meaning
// (functions.md § 38A(5)(7); unrolling stays an optimization). An empty
// pack never enters the body. false means stmt is not a pack loop.
func (fb *functionBuilder) buildPackFor(stmt *ast.ForStatement) (bool, error) {
	spec := fb.specialization
	identifier, ok := stmt.Iterable.(*ast.Identifier)
	if !ok || spec == nil || spec.facts == nil || len(stmt.Bindings) == 0 {
		return false, nil
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok || fact.ID != spec.binding || spec.binding == 0 {
		return false, nil
	}
	pack, ok := spec.substitution[spec.facts.Parameter]
	if !ok || len(pack.TypeArgs) != len(spec.facts.Types) {
		return true, fb.unsupported("pack loop without the specialization's ordered element types", stmt.Token)
	}
	count := len(spec.facts.Types)
	offset := len(fb.fn.Parameters) - count
	if offset < 0 {
		return true, fb.unsupported("pack loop without expanded pack parameters", stmt.Token)
	}
	valueBinding := stmt.Bindings[len(stmt.Bindings)-1]
	loc := location(stmt.Token)
	boolType, err := fb.boolType()
	if err != nil {
		return true, err
	}
	cursorType, err := fb.owner.internType(sema.Type{Name: "uint", Kind: sema.UintType})
	if err != nil {
		return true, err
	}
	loop := &activeLoop{id: fb.nextLoop}
	fb.nextLoop++
	record := LoopRecord{ID: loop.id, Kind: LoopForPack, ContinuesAfter: true, Location: loc}
	if count == 0 {
		record.ConditionKnown, record.ConditionValue = true, false
	}
	fb.fn.Loops = append(fb.fn.Loops, record)
	recordIndex := len(fb.fn.Loops) - 1

	zero := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(0), Location: loc}, cursorType)
	cursor := fb.forStorage(valueBinding.Name+".position", cursorType, zero, loc)
	loop.condition = fb.newBlock()
	record.ConditionBlock = loop.condition.ID
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeEnter, Location: loc})
	fb.current = loop.condition
	exit := fb.loopExit(loop)
	if count == 0 {
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: exit.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})
		record.ExitBlock = exit.ID
		fb.fn.Loops[recordIndex] = record
		fb.current = exit
		return true, nil
	}
	position := fb.result(Operation{Kind: OpStorageLoad, Storage: cursor, Location: loc}, cursorType)
	length := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(int64(count)), Location: loc}, cursorType)
	more := fb.forCompare(IntegerCompareLT, position, length, boolType, loc)
	body := fb.newBlock()
	record.BodyBlock = body.ID
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{more.id}, Successors: []BranchTarget{{Block: body.ID}, {Block: exit.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})

	// Yield the position, then advance the cursor before user code runs, so
	// `continue` returns straight to the condition.
	fb.current = body
	current := fb.result(Operation{Kind: OpStorageLoad, Storage: cursor, Location: loc}, cursorType)
	one := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(1), Location: loc}, cursorType)
	next, err := fb.forAdd(current, one, cursorType, loc)
	if err != nil {
		return true, err
	}
	fb.emit(Operation{Kind: OpStorageStore, Storage: cursor, Operands: []ValueID{next.id}, Location: loc})

	element := sema.TypePackElementName(spec.facts.Parameter, valueBinding.Name)
	outer := fb.owner.substitution
	defer func() { fb.owner.substitution = outer }()
	for k := 0; k < count; k++ {
		positionBody := fb.newBlock()
		if k < count-1 {
			constant := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(int64(k)), Location: loc}, cursorType)
			selected := fb.forCompare(IntegerCompareEQ, current, constant, boolType, loc)
			nextTest := fb.newBlock()
			fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{selected.id}, Successors: []BranchTarget{{Block: positionBody.ID}, {Block: nextTest.ID}}, Location: loc})
			fb.current = positionBody
			if err := fb.buildPackPosition(stmt, k, offset, element, pack.TypeArgs[k], outer, loop, loc); err != nil {
				return true, err
			}
			fb.current = nextTest
			continue
		}
		// The last position needs no test: the condition proved cursor < N.
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: positionBody.ID}}, Location: loc})
		fb.current = positionBody
		if err := fb.buildPackPosition(stmt, k, offset, element, pack.TypeArgs[k], outer, loop, loc); err != nil {
			return true, err
		}
	}
	record.ExitBlock = exit.ID
	fb.fn.Loops[recordIndex] = record
	fb.current = exit
	return true, nil
}

// buildPackPosition builds the loop body once for position k.
func (fb *functionBuilder) buildPackPosition(stmt *ast.ForStatement, k, offset int, element string, concrete sema.Type, outer map[string]sema.Type, loop *activeLoop, loc Location) error {
	substitution := make(map[string]sema.Type, len(outer)+1)
	for name, typ := range outer {
		substitution[name] = typ
	}
	substitution[element] = concrete
	fb.owner.substitution = substitution
	parameter := fb.fn.Parameters[offset+k]
	if len(stmt.Bindings) == 2 && !stmt.Bindings[0].Discard {
		if err := fb.bindPackLoopIndex(stmt.Bindings[0], k, loc); err != nil {
			return err
		}
	}
	valueBinding := stmt.Bindings[len(stmt.Bindings)-1]
	if !valueBinding.Discard {
		fact, ok := fb.owner.analyzer.ResolvedBindingDeclaredAt(valueBinding.Token.File, valueBinding.Token.Line, valueBinding.Token.Column)
		if !ok {
			return fb.unsupported("unresolved pack loop binding "+valueBinding.Name, valueBinding.Token)
		}
		bound, err := fb.packLoopElement(stmt, valueBinding, parameter, k, loc)
		if err != nil {
			return err
		}
		fb.bindings[fact.ID] = binding{value: bound.id, typ: bound.typ}
	}
	fb.loops = append(fb.loops, loop)
	err := fb.buildScope(stmt.Body)
	fb.loops = fb.loops[:len(fb.loops)-1]
	if err != nil {
		return err
	}
	if fb.current != nil {
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeBack, Location: loc})
	}
	return nil
}

// packLoopElement yields what a position's loop binding holds. A reference
// position is its expanded parameter, whose uses copy or reborrow it; `for ref`
// over an owned position borrows the callee-owned value through a read-only
// Place root with Sema's plan.
func (fb *functionBuilder) packLoopElement(stmt *ast.ForStatement, valueBinding ast.ForBinding, parameter Parameter, k int, loc Location) (builtValue, error) {
	owned := fb.specialization.facts.Positions[k].Mode == TypePackOwned
	if !owned || valueBinding.Mode != ast.ForBindingRef {
		if parameter.Reference != nil {
			fb.recordReference(parameter.Value.ID, parameter.Reference)
		}
		return builtValue{id: parameter.Value.ID, typ: parameter.Value.Type}, nil
	}
	plan, ok := fb.owner.analyzer.ResolvedTypePackLoopBorrowOf(stmt)
	if !ok {
		return builtValue{}, fb.unsupported("for ref over an owned pack position without a resolved borrow", valueBinding.ModeToken)
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: parameter.Value.Type})
	place := fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{parameter.Value.ID}, Place: placeMetadataOf(plan.Place), Location: loc}, placeType)
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: parameter.Value.Type})
	value := fb.newValue(referenceType, OwnershipSharedReference, loc)
	metadata := referenceMetadataOf(plan.Facts)
	fb.emit(Operation{Kind: OpReferenceBorrowShared, Operands: []ValueID{place.id}, Results: []Value{value}, Reference: metadata, Location: loc})
	fb.recordReference(value.ID, metadata)
	return builtValue{id: value.ID, typ: referenceType}, nil
}

// bindPackLoopIndex binds the index binding of position k to its constant.
func (fb *functionBuilder) bindPackLoopIndex(index ast.ForBinding, k int, loc Location) error {
	fact, ok := fb.owner.analyzer.ResolvedBindingDeclaredAt(index.Token.File, index.Token.Line, index.Token.Column)
	if !ok {
		return fb.unsupported("unresolved pack loop index "+index.Name, index.Token)
	}
	typeID, err := fb.owner.internType(fact.Type)
	if err != nil {
		return err
	}
	value := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(int64(k)), Location: loc}, typeID)
	fb.bindings[fact.ID] = binding{value: value.id, typ: typeID}
	return nil
}

// packIndexValue is the expanded parameter a constant `values[k]` selects.
func (fb *functionBuilder) packIndexValue(expr *ast.IndexExpression) (builtValue, bool) {
	spec := fb.specialization
	if spec == nil || spec.facts == nil {
		return builtValue{}, false
	}
	position, ok := fb.owner.analyzer.ResolvedTypePackIndexOf(expr)
	offset := len(fb.fn.Parameters) - len(spec.facts.Types)
	if !ok || position >= len(spec.facts.Types) || offset < 0 {
		return builtValue{}, false
	}
	parameter := fb.fn.Parameters[offset+position]
	if parameter.Reference != nil {
		fb.recordReference(parameter.Value.ID, parameter.Reference)
	}
	return builtValue{id: parameter.Value.ID, typ: parameter.Value.Type}, true
}

// buildPackForward lowers a call that expands the pack of the specialization
// being built. Sema resolves the callee for these concrete elements; each
// expanded argument is an ordinary operand, with its pack position recorded. A
// copy needs no storage, and a move of a trivial element is one too: the source
// pack ends in Sema, whose facts the operation keeps (§ 27A(7)).
func (fb *functionBuilder) buildPackForward(call *ast.CallExpression) (builtValue, error) {
	spec := fb.specialization
	if spec == nil || spec.facts == nil {
		return builtValue{}, fb.unsupported("pack forwarding call outside a pack specialization", call.Token)
	}
	resolved, problem := fb.owner.analyzer.ResolvedTypePackForwardOf(call, fb.owner.substitution)
	if problem != "" {
		return builtValue{}, fb.unsupported(problem, call.Token)
	}
	callee, parameters, returnType := functionID(resolved.Function, fb.owner.module.Types), resolved.Function.Parameters, resolved.Function.ReturnType
	if isGenericInstance(resolved.Function) {
		target, err := fb.owner.demandSpecialization(call, resolved.Function)
		if err != nil {
			return builtValue{}, err
		}
		callee, parameters, returnType = target.id, target.function.Parameters, target.function.ReturnType
	}
	if len(parameters) != len(resolved.Expanded) {
		return builtValue{}, fb.unsupported("pack forwarding into a homogeneous variadic parameter", call.Token)
	}
	facts := &PackForwardFacts{Parameter: spec.facts.Parameter, Consuming: resolved.Consuming}
	operands := make([]ValueID, 0, len(parameters)+1)
	actions := make([]ArgumentAction, 0, len(parameters)+1)
	if !resolved.Function.Static && resolved.Function.ImplTarget != "" {
		if !fb.owner.instanceMethodLowerable(resolved.Function) {
			return builtValue{}, fb.unsupported("instance method call", call.Token)
		}
		receiver, action, err := fb.buildReceiverArgument(call, resolved.Function)
		if err != nil {
			return builtValue{}, err
		}
		operands, actions = append(operands, receiver.id), append(actions, action)
		facts.Positions = append(facts.Positions, -1)
	} else if resolved.Function.ImplTarget != "" && !staticMethodLowerable(resolved.Function) {
		return builtValue{}, fb.unsupported("static method of a generic implementation", call.Token)
	}
	for index, argument := range resolved.Expanded {
		expected, err := fb.owner.internType(parameters[index].Type)
		if err != nil {
			return builtValue{}, err
		}
		action, isReference := fb.referenceArgumentAction(expected)
		var value builtValue
		if argument.Position < 0 {
			value, err = fb.buildExpr(call.Arguments[argument.Source], expected)
			if err == nil && !isReference && !fb.storageAllowed(value.typ) {
				err = fb.unsupported("non-trivial call argument", expressionToken(call.Arguments[argument.Source]))
			}
		} else {
			value, err = fb.packForwardOperand(call, resolved, index, argument.Position, isReference)
		}
		if err != nil {
			return builtValue{}, err
		}
		if value.typ != expected {
			return builtValue{}, fb.unsupported("pack forwarding argument whose type differs from its parameter", call.Token)
		}
		if !isReference {
			action = ArgumentCopyTrivial
		}
		operands = append(operands, value.id)
		actions = append(actions, action)
		facts.Positions = append(facts.Positions, argument.Position)
	}
	ret, err := fb.owner.internSignatureType(returnType)
	if err != nil {
		return builtValue{}, err
	}
	op := Operation{Kind: OpDirectCall, Callee: callee, Operands: operands, ArgumentActions: actions, PackForward: facts, Location: location(call.Token)}
	if call.HasUnsafePrefix() {
		marker := location(call.Unsafe)
		op.UnsafeAuthorization = &marker
	}
	if returnType.Kind == sema.VoidType {
		fb.emit(op)
		return builtValue{typ: ret}, nil
	}
	if returnType.Kind == sema.ReferenceType {
		// The caller-side facts Sema instantiated over the pack's origins.
		referenceFacts, ok := fb.owner.analyzer.ResolvedCallReferenceOf(call)
		if !ok || !referenceFacts.Checkable() {
			return builtValue{}, fb.unsupported("pack forwarding call returning a reference without an instantiated origin", call.Token)
		}
		ownership := OwnershipSharedReference
		if returnType.ReferenceMutable {
			ownership = OwnershipMutableReference
		}
		metadata := referenceMetadataOf(referenceFacts)
		value := fb.newValue(ret, ownership, op.Location)
		op.Results, op.Reference = []Value{value}, metadata
		fb.emit(op)
		fb.recordReference(value.ID, metadata)
		return builtValue{id: value.ID, typ: ret}, nil
	}
	return fb.result(op, ret), nil
}

// packForwardOperand yields position k of the enclosing pack for one expanded
// argument: the expanded parameter itself for a copied element, a shared loan
// of a callee-owned element, or a copy or reborrow of a held reference with its
// own borrow identity.
func (fb *functionBuilder) packForwardOperand(call *ast.CallExpression, resolved sema.ResolvedTypePackForward, index, k int, toReference bool) (builtValue, error) {
	offset := len(fb.fn.Parameters) - len(fb.specialization.facts.Types)
	if offset < 0 || k >= len(fb.specialization.facts.Types) {
		return builtValue{}, fb.unsupported("pack forwarding without expanded pack parameters", call.Token)
	}
	parameter := fb.fn.Parameters[offset+k]
	source := builtValue{id: parameter.Value.ID, typ: parameter.Value.Type}
	loc := location(call.Token)
	borrow, lifetime := fb.owner.analyzer.TypePackForwardBorrowIdentity(call, index)
	if !toReference {
		if !fb.storageAllowed(source.typ) {
			return builtValue{}, fb.unsupported("ownership-sensitive owned pack forwarding", call.Token)
		}
		return source, nil
	}
	if parameter.Reference == nil {
		placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: source.typ})
		place := fb.placeResult(Operation{Kind: OpPlaceValueRoot, Operands: []ValueID{source.id}, Place: placeMetadataOf(resolved.Borrow.Place), Location: loc}, placeType)
		referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: source.typ})
		value := fb.newValue(referenceType, OwnershipSharedReference, loc)
		metadata := referenceMetadataOf(resolved.Borrow.Facts)
		metadata.Borrow, metadata.Lifetime = uint32(borrow), uint32(lifetime)
		fb.emit(Operation{Kind: OpReferenceBorrowShared, Operands: []ValueID{place.id}, Results: []Value{value}, Reference: metadata, Location: loc})
		fb.recordReference(value.ID, metadata)
		return builtValue{id: value.ID, typ: referenceType}, nil
	}
	sourceType, ok := fb.owner.module.Types.Lookup(source.typ)
	if !ok || sourceType.Kind != TypeReference {
		return builtValue{}, fmt.Errorf("pack position %d does not hold a reference", k)
	}
	parameterType := resolved.Expanded[index].Parameter.Type
	derived := *parameter.Reference
	derived.Borrow, derived.Lifetime = uint32(borrow), uint32(lifetime)
	return fb.passReference(source, sourceType, parameter.Reference, &derived, parameterType.ReferenceMutable, call.Token)
}

// explicitMoveFree strips the `<-` marker of a call argument.
func explicitMoveFree(argument ast.Expression) ast.Expression {
	if move, ok := argument.(*ast.PrefixExpression); ok && move.Operator == "<-" {
		return move.Right
	}
	return argument
}

// formatPackForwardFacts prints the provenance facts of a forwarding call.
func formatPackForwardFacts(facts *PackForwardFacts) string {
	positions := make([]string, len(facts.Positions))
	for index, position := range facts.Positions {
		positions[index] = fmt.Sprint(position)
	}
	mode := "spread"
	if facts.Consuming {
		mode = "consuming-spread"
	}
	return fmt.Sprintf(" pack-%s(%s positions=[%s])", mode, facts.Parameter, strings.Join(positions, ", "))
}

// verifyPackForwardFacts checks a forwarding call against the enclosing pack:
// one entry per operand, positions in range, and a whole-pack transfer only from
// a pack of owned elements, whose operands are exactly its expanded parameters.
func verifyPackForwardFacts(fn *Function, op *Operation) error {
	facts := op.PackForward
	if facts == nil {
		return nil
	}
	if fn.TypePack == nil || facts.Parameter != fn.TypePack.Parameter || len(facts.Positions) != len(op.Operands) {
		return fmt.Errorf("function %s has inconsistent pack forwarding facts", fn.ID)
	}
	for index, position := range facts.Positions {
		if position == -1 {
			continue
		}
		if position < 0 || position >= len(fn.TypePack.Types) {
			return fmt.Errorf("function %s forwards pack position %d outside its pack", fn.ID, position)
		}
		if facts.Consuming {
			if fn.TypePack.Positions[position].Mode != TypePackOwned || op.Operands[index] != fn.Parameters[fn.TypePack.FixedCount+position].Value.ID {
				return fmt.Errorf("function %s moves pack position %d, which it does not own", fn.ID, position)
			}
		}
	}
	return nil
}
