package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// buildFor lowers the `for` forms the single compilation track supports to
// the explicit loop CFG of buildWhile (semantic_ir.md § 65(1)–(2), § 66):
//
//   - `for { }` is an infinite loop whose condition is constantly true; it
//     continues after the loop only through a reachable `break`;
//   - `for i in a..<b` and `for i in a..b` over a built-in integer type with
//     the implicit +1 step or a compile-time constant step evaluate both
//     bounds exactly once and keep the
//     progression in compiler-generated storage: a cursor holding the next
//     value and a flag stating whether one remains. The body entry yields the
//     cursor, then advances it before user code runs, so `continue` returns
//     straight to the condition. The cursor is advanced only when a further
//     value exists, so an inclusive range ending at the type's maximum never
//     needs an overflowing increment (flowcontrol_for.md § 29).
//
// With an explicit step the remaining distance to end is computed exactly in
// a wider integer type (forWideDistance), so the method does not depend on
// the element type's target-resolved limits. Other forms (runtime steps,
// collections, strings, Iterator[T], reference bindings, named numeric
// types) remain explicit unsupported constructs.
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
	step := big.NewInt(1)
	if stmt.Step != nil {
		value, ok := fb.owner.analyzer.ResolvedForRangeStepOf(stmt)
		if !ok {
			return fb.unsupported("for range with a step that is not a compile-time constant", stmt.StepToken)
		}
		step = value
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
	ascending := step.Sign() > 0
	firstTest := map[[2]bool]IntegerComparePredicate{
		{true, false}: IntegerCompareLE, {true, true}: IntegerCompareLT,
		{false, false}: IntegerCompareGE, {false, true}: IntegerCompareGT,
	}[[2]bool{ascending, rangeExpr.Exclusive}]
	var wide *forWideDistance
	if step.Cmp(big.NewInt(1)) != 0 {
		if wide, err = fb.forWideDistanceSetup(elementType, end, step, loc); err != nil {
			return err
		}
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
	one := fb.result(Operation{Kind: OpConstInt, Integer: new(big.Int).Set(step), Location: loc}, typeID)
	if wide != nil {
		// § 29 with an explicit step: a further value exists when the
		// remaining distance to end, computed exactly in a wider type, reaches
		// |step| (inclusive) or exceeds it (exclusive); only then is the
		// cursor advanced, so no increment past end is ever performed.
		hasNext, err := fb.forHasNextWide(wide, value, ascending, rangeExpr.Exclusive, boolType, loc)
		if err != nil {
			return err
		}
		fb.emit(Operation{Kind: OpStorageStore, Storage: more, Operands: []ValueID{hasNext.id}, Location: loc})
		advance := fb.newBlock()
		run := fb.newBlock()
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{hasNext.id}, Successors: []BranchTarget{{Block: advance.ID}, {Block: run.ID}}, Location: loc})
		fb.current = advance
		next, err := fb.forAdd(value, one, typeID, loc)
		if err != nil {
			return err
		}
		fb.emit(Operation{Kind: OpStorageStore, Storage: cursor, Operands: []ValueID{next.id}, Location: loc})
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: run.ID}}, Location: loc})
		fb.current = run
	} else if rangeExpr.Exclusive {
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

// forWideDistance holds the loop-invariant parts of an explicit-step range:
// the end bound and |step| in a type wide enough that the distance between
// any two values of the element type is exact.
type forWideDistance struct {
	wideType TypeID
	wideEnd  builtValue
	absStep  builtValue
}

// forWideType is the exact distance domain of a range element type: int128
// for every integer up to 64 bits (including target-sized int/uint) and
// int256 for 128-bit integers.
func forWideType(element sema.Type) (string, bool) {
	switch element.Name {
	case "int", "uint", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64", "byte":
		return "int128", true
	case "int128", "uint128":
		return "int256", true
	}
	return "", false
}

func (fb *functionBuilder) forWideDistanceSetup(element sema.Type, end builtValue, step *big.Int, loc Location) (*forWideDistance, error) {
	name, ok := forWideType(element)
	if !ok {
		return nil, fb.unsupported("for range step over "+element.Name, lexer.Token{})
	}
	wideSema, ok := fb.owner.analyzer.Types()[name]
	if !ok {
		return nil, fmt.Errorf("missing built-in type %s", name)
	}
	wideType, err := fb.owner.internType(wideSema)
	if err != nil {
		return nil, err
	}
	wideEnd := fb.result(Operation{Kind: OpNumericConvertExact, Operands: []ValueID{end.id}, Location: loc}, wideType)
	absStep := fb.result(Operation{Kind: OpConstInt, Integer: new(big.Int).Abs(step), Location: loc}, wideType)
	return &forWideDistance{wideType: wideType, wideEnd: wideEnd, absStep: absStep}, nil
}

func (fb *functionBuilder) forHasNextWide(wide *forWideDistance, value builtValue, ascending, exclusive bool, boolType TypeID, loc Location) (builtValue, error) {
	wideValue := fb.result(Operation{Kind: OpNumericConvertExact, Operands: []ValueID{value.id}, Location: loc}, wide.wideType)
	left, right := wide.wideEnd, wideValue
	if !ascending {
		left, right = wideValue, wide.wideEnd
	}
	op := Operation{Kind: OpIntBinaryChecked, IntegerBinary: IntegerCheckedSubtract, Operator: "-", Operands: []ValueID{left.id, right.id}, Location: loc}
	distance, err := fb.emitCheckedOperator(op, wide.wideType, nil, nil, nil)
	if err != nil {
		return builtValue{}, err
	}
	predicate := IntegerCompareGE
	if exclusive {
		predicate = IntegerCompareGT
	}
	return fb.forCompare(predicate, distance, wide.absStep, boolType, loc), nil
}
