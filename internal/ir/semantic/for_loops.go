package semantic

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildFor lowers the `for` forms the single compilation track supports to
// the explicit loop CFG of buildWhile (semantic_ir.md § 65(1)–(2), § 66):
//
//   - `for { }` is an infinite loop whose condition is constantly true; it
//     continues after the loop only through a reachable `break`;
//   - `for i in a..<b` and `for i in a..b` over a built-in integer type with
//     the implicit +1 step evaluate both bounds exactly once and keep the
//     progression in compiler-generated storage: a cursor holding the next
//     value and a flag stating whether one remains. The body entry yields the
//     cursor, then advances it before user code runs, so `continue` returns
//     straight to the condition. The cursor is advanced only when a further
//     value exists, so an inclusive range ending at the type's maximum never
//     needs an overflowing increment (flowcontrol_for.md § 29).
//
// Other forms (explicit step, collections, strings, Iterator[T], reference
// bindings, named numeric types) remain explicit unsupported constructs.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — §§ 2, 9, 22–25, 29–31
//   - rules/compiler/semantic_ir.md — § 65(1)–(3), § 66(1)
func (fb *functionBuilder) buildFor(stmt *ast.ForStatement) error {
	if stmt.Iterable == nil && len(stmt.Bindings) == 0 {
		return fb.buildInfiniteFor(stmt)
	}
	rangeExpr, ok := stmt.Iterable.(*ast.RangeExpression)
	if !ok {
		return fb.unsupported("for iteration over a collection, string or iterator", stmt.Token)
	}
	if stmt.Step != nil {
		return fb.unsupported("for range with an explicit step", stmt.StepToken)
	}
	if len(stmt.Bindings) != 1 || stmt.Bindings[0].Mode != ast.ForBindingValue {
		return fb.unsupported("for range binding form", stmt.Token)
	}
	if rangeExpr.Start == nil || rangeExpr.End == nil {
		return fb.unsupported("open-ended for range", rangeExpr.Token)
	}
	loopBinding := stmt.Bindings[0]
	elementType, err := fb.forRangeElementType(stmt, rangeExpr)
	if err != nil {
		return err
	}
	if elementType.Kind != sema.IntType && elementType.Kind != sema.UintType || elementType.Named {
		return fb.unsupported("for range over "+elementType.Name, rangeExpr.Token)
	}
	typeID, err := fb.owner.internType(elementType)
	if err != nil {
		return err
	}
	boolType, err := fb.boolType()
	if err != nil {
		return err
	}
	loc := location(stmt.Token)

	// § 23: each bound is evaluated exactly once, start before end.
	start, err := fb.buildExpr(rangeExpr.Start, typeID)
	if err != nil {
		return err
	}
	end, err := fb.buildExpr(rangeExpr.End, typeID)
	if err != nil {
		return err
	}
	if start.typ != typeID || end.typ != typeID {
		return fb.unsupported("for range bounds of different representation", rangeExpr.Token)
	}
	firstTest := IntegerCompareLE
	if rangeExpr.Exclusive {
		firstTest = IntegerCompareLT
	}
	hasFirst := fb.forCompare(firstTest, start, end, boolType, loc)
	cursor := fb.forStorage(loopBinding.Name+".cursor", typeID, start, loc)
	more := fb.forStorage(loopBinding.Name+".more", boolType, hasFirst, loc)

	loop := &activeLoop{id: fb.nextLoop}
	fb.nextLoop++
	record := LoopRecord{ID: loop.id, Kind: LoopForRange, ContinuesAfter: true, Location: loc}
	fb.fn.Loops = append(fb.fn.Loops, record)
	recordIndex := len(fb.fn.Loops) - 1

	loop.condition = fb.newBlock()
	record.ConditionBlock = loop.condition.ID
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeEnter, Location: loc})
	fb.current = loop.condition
	remaining := fb.result(Operation{Kind: OpStorageLoad, Storage: more, Location: loc}, boolType)
	body := fb.newBlock()
	record.BodyBlock = body.ID
	exit := fb.loopExit(loop)
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{remaining.id}, Successors: []BranchTarget{{Block: body.ID}, {Block: exit.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})

	// Yield the cursor, then advance the progression for the next attempt.
	fb.current = body
	value := fb.result(Operation{Kind: OpStorageLoad, Storage: cursor, Location: loc}, typeID)
	if !loopBinding.Discard {
		fact, ok := fb.owner.analyzer.ResolvedBindingDeclaredAt(loopBinding.Token.File, loopBinding.Token.Line, loopBinding.Token.Column)
		if !ok {
			return fb.unsupported("unresolved for binding "+loopBinding.Name, loopBinding.Token)
		}
		fb.bindings[fact.ID] = binding{value: value.id, typ: typeID}
	}
	one := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(1), Location: loc}, typeID)
	if rangeExpr.Exclusive {
		// value < end, so value + 1 cannot overflow.
		next, err := fb.forAdd(value, one, typeID, loc)
		if err != nil {
			return err
		}
		fb.emit(Operation{Kind: OpStorageStore, Storage: cursor, Operands: []ValueID{next.id}, Location: loc})
		stillLess := fb.forCompare(IntegerCompareLT, next, end, boolType, loc)
		fb.emit(Operation{Kind: OpStorageStore, Storage: more, Operands: []ValueID{stillLess.id}, Location: loc})
	} else {
		belowEnd := fb.forCompare(IntegerCompareLT, value, end, boolType, loc)
		fb.emit(Operation{Kind: OpStorageStore, Storage: more, Operands: []ValueID{belowEnd.id}, Location: loc})
		advance := fb.newBlock()
		run := fb.newBlock()
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{belowEnd.id}, Successors: []BranchTarget{{Block: advance.ID}, {Block: run.ID}}, Location: loc})
		fb.current = advance
		next, err := fb.forAdd(value, one, typeID, loc)
		if err != nil {
			return err
		}
		fb.emit(Operation{Kind: OpStorageStore, Storage: cursor, Operands: []ValueID{next.id}, Location: loc})
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: run.ID}}, Location: loc})
		fb.current = run
	}

	fb.loops = append(fb.loops, loop)
	err = fb.buildStatements(stmt.Body.Statements)
	fb.loops = fb.loops[:len(fb.loops)-1]
	if err != nil {
		return err
	}
	if fb.current != nil {
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeBack, Location: loc})
	}
	record.ExitBlock = loop.exit.ID
	fb.current = loop.exit
	fb.fn.Loops[recordIndex] = record
	return nil
}

// buildInfiniteFor lowers `for { }`: a constantly true condition, left only
// through `break` (flowcontrol_for.md § 2; semantic_ir.md § 65(2)).
func (fb *functionBuilder) buildInfiniteFor(stmt *ast.ForStatement) error {
	loc := location(stmt.Token)
	loop := &activeLoop{id: fb.nextLoop}
	fb.nextLoop++
	record := LoopRecord{ID: loop.id, Kind: LoopForInfinite, ConditionKnown: true, ConditionValue: true, Location: loc}
	fb.fn.Loops = append(fb.fn.Loops, record)
	recordIndex := len(fb.fn.Loops) - 1

	loop.condition = fb.newBlock()
	record.ConditionBlock = loop.condition.ID
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeEnter, Location: loc})
	fb.current = loop.condition
	body := fb.newBlock()
	record.BodyBlock = body.ID
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: body.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})
	fb.current = body
	fb.loops = append(fb.loops, loop)
	err := fb.buildStatements(stmt.Body.Statements)
	fb.loops = fb.loops[:len(fb.loops)-1]
	if err != nil {
		return err
	}
	if fb.current != nil {
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeBack, Location: loc})
	}
	fb.current = nil
	if loop.exit != nil {
		// Only a reachable `break` creates the exit.
		record.ExitBlock = loop.exit.ID
		record.ContinuesAfter = true
		fb.current = loop.exit
	}
	fb.fn.Loops[recordIndex] = record
	return nil
}

// forRangeElementType is Sema's binding type, or for a discard binding the
// type Sema assigned to the start bound.
func (fb *functionBuilder) forRangeElementType(stmt *ast.ForStatement, rangeExpr *ast.RangeExpression) (sema.Type, error) {
	binding := stmt.Bindings[0]
	if !binding.Discard {
		fact, ok := fb.owner.analyzer.ResolvedBindingDeclaredAt(binding.Token.File, binding.Token.Line, binding.Token.Column)
		if !ok {
			return sema.Type{}, fb.unsupported("unresolved for binding "+binding.Name, binding.Token)
		}
		return fact.Type, nil
	}
	typ, ok := fb.owner.analyzer.ResolvedTypeOf(rangeExpr.Start)
	if !ok {
		return sema.Type{}, fb.unsupported("for range with an unresolved bound type", rangeExpr.Token)
	}
	return typ, nil
}

func (fb *functionBuilder) forCompare(predicate IntegerComparePredicate, left, right builtValue, boolType TypeID, loc Location) builtValue {
	return fb.result(Operation{Kind: OpIntCompare, IntegerCompare: predicate, Operands: []ValueID{left.id, right.id}, Location: loc}, boolType)
}

func (fb *functionBuilder) forAdd(left, right builtValue, typeID TypeID, loc Location) (builtValue, error) {
	op := Operation{Kind: OpIntBinaryChecked, IntegerBinary: IntegerCheckedAdd, Operator: "+", Operands: []ValueID{left.id, right.id}, Location: loc}
	return fb.emitCheckedOperator(op, typeID, nil, nil, nil)
}

func (fb *functionBuilder) forStorage(name string, typeID TypeID, initial builtValue, loc Location) StorageID {
	id := fb.nextStorage
	fb.nextStorage++
	fb.fn.Storages = append(fb.fn.Storages, Storage{ID: id, Name: name, Type: typeID, Mutable: true, Class: StorageLocalAutomatic, Location: loc})
	fb.emit(Operation{Kind: OpStorageDeclare, Storage: id, Location: loc})
	fb.emit(Operation{Kind: OpStorageInit, Storage: id, Operands: []ValueID{initial.id}, Location: loc})
	return id
}
