package semantic

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
)

// LoopForPack is a `for` over a heterogeneous type pack in one
// specialization (semantic_ir.md § 27A; flowcontrol_for.md § 13A).
const LoopForPack LoopKind = "for-pack"

// buildPackFor lowers a `for` over the heterogeneous pack of the
// specialization being built. The template body was checked once with the
// opaque element `Types[i]`; here every position k gets its own copy of the
// body built with `Types[i]` bound to the concrete element k and the loop
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
	if valueBinding.Mode == ast.ForBindingRef && !packElementIsReference(spec) {
		return true, fb.unsupported("for ref over an owned pack position (needs a borrow of the callee-owned element)", valueBinding.ModeToken)
	}
	if valueBinding.Mode == ast.ForBindingRefMut {
		return true, fb.unsupported("for ref mut pack loop (per-position exclusive reborrow facts are not produced yet)", valueBinding.ModeToken)
	}
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

	element := sema.TypePackElementName(spec.facts.Parameter)
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
		fb.bindings[fact.ID] = binding{value: parameter.Value.ID, typ: parameter.Value.Type}
		if parameter.Reference != nil {
			fb.recordReference(parameter.Value.ID, parameter.Reference)
		}
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

// packElementIsReference reports a `...ref Types` / `...ref mut Types`
// specialization, whose positions are reference parameters.
func packElementIsReference(spec *specialization) bool {
	for _, position := range spec.facts.Positions {
		if position.Mode == TypePackOwned {
			return false
		}
	}
	return true
}
