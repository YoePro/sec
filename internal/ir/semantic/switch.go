package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// switchCaseBody is one clause body in source order; default is last.
type switchCaseBody struct {
	clause *ast.SwitchCase
	block  *Block
}

// buildSwitch lowers a subject switch over a built-in integer and a
// subjectless switch to explicit comparisons and branches
// (semantic_ir.md § 68(2)). The subject is evaluated exactly once; clauses
// are tested in source order and their comma alternatives left to right,
// each alternative evaluated only when control reaches it; the first match
// enters its body; a body's normal end continues after the switch and an
// explicit `fallthrough` enters the next body without testing it; `default`
// runs only after every test failed. When Sema proved there is no unmatched
// path and no `default` exists, the final failed test reaches a synthesized
// unreachable. Value alternatives test equality, relational alternatives
// apply their ordered comparison, and range alternatives evaluate their
// bounds in order before testing membership (flowcontrol_switch.md §§ 4–17,
// § 39).
//
// Enum subjects test each case with same-enum equality and bool subjects with
// branch-based bool equality; string and named-type subjects remain
// unsupported constructs.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §§ 4–17, 31–32, 39
//   - rules/compiler/semantic_ir.md — § 68
func (fb *functionBuilder) buildSwitch(stmt *ast.SwitchStatement) error {
	flow, ok := fb.owner.analyzer.ResolvedSwitchFlowOf(stmt)
	if !ok {
		return fmt.Errorf("missing resolved switch flow at %s", formatLocation(location(stmt.Token)))
	}
	loc := location(stmt.Token)
	boolType, err := fb.boolType()
	if err != nil {
		return err
	}
	var subject builtValue
	if flow.HasSubject {
		subjectType := flow.SubjectType
		isInteger := (subjectType.Kind == sema.IntType || subjectType.Kind == sema.UintType) && !subjectType.Named
		if !isInteger && subjectType.Kind != sema.EnumType && subjectType.Kind != sema.BoolType {
			return fb.unsupported("switch over "+typeName(subjectType), stmt.Token)
		}
		typeID, err := fb.owner.internType(subjectType)
		if err != nil {
			return err
		}
		if subject, err = fb.buildExpr(stmt.Subject, typeID); err != nil {
			return err
		}
		if subject.typ != typeID {
			return fb.unsupported("switch subject of a different representation", stmt.Token)
		}
	}

	bodies := []switchCaseBody{}
	for _, clause := range stmt.Cases {
		bodies = append(bodies, switchCaseBody{clause: clause, block: fb.newBlock()})
	}
	if stmt.Default != nil {
		bodies = append(bodies, switchCaseBody{clause: stmt.Default, block: fb.newBlock()})
	}
	var merge *Block
	mergeBlock := func() *Block {
		if merge == nil {
			merge = fb.newBlock()
		}
		return merge
	}
	// noMatch is where control goes when every case test failed.
	noMatch := func() {
		switch {
		case stmt.Default != nil:
			fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: bodies[len(bodies)-1].block.ID}}, Location: loc})
		case flow.HasUnmatchedPath:
			fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: mergeBlock().ID}}, Location: loc})
		default:
			fb.emit(Operation{Kind: OpUnreachable, Synthesized: true, Reason: "switch-unmatched-excluded-by-sema", Location: loc})
		}
	}

	// Test chain: each failed alternative continues with the next one.
	for index, clause := range stmt.Cases {
		for _, item := range clause.Items {
			next := fb.newBlock()
			if err := fb.switchItemTest(item, subject, flow.HasSubject, boolType, bodies[index].block, next); err != nil {
				return err
			}
			fb.current = next
		}
	}
	noMatch()

	// Bodies: normal completion continues after the switch; fallthrough
	// enters the next body directly.
	for index, body := range bodies {
		fb.current = body.block
		next := (*Block)(nil)
		if index+1 < len(bodies) {
			next = bodies[index+1].block
		}
		fb.fallthroughTargets = append(fb.fallthroughTargets, next)
		err := fb.buildStatements(body.clause.Body.Statements)
		fb.fallthroughTargets = fb.fallthroughTargets[:len(fb.fallthroughTargets)-1]
		if err != nil {
			return err
		}
		if fb.current != nil {
			fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: mergeBlock().ID}}, Location: loc})
		}
	}
	fb.current = merge
	return nil
}

// buildFallthrough transfers to the next clause body of the innermost switch
// (flowcontrol_switch.md § 16); Sema already validated its position.
func (fb *functionBuilder) buildFallthrough(token lexer.Token) error {
	if len(fb.fallthroughTargets) == 0 || fb.fallthroughTargets[len(fb.fallthroughTargets)-1] == nil {
		return fb.unsupported("fallthrough without a following switch case", token)
	}
	target := fb.fallthroughTargets[len(fb.fallthroughTargets)-1]
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: target.ID}}, Location: location(token)})
	fb.current = nil
	return nil
}

// switchItemTest branches to body when one case alternative matches and to
// next otherwise.
func (fb *functionBuilder) switchItemTest(item ast.SwitchCaseItem, subject builtValue, hasSubject bool, boolType TypeID, body, next *Block) error {
	branch := func(condition builtValue, token lexer.Token, matched, failed *Block) {
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{condition.id}, Successors: []BranchTarget{{Block: matched.ID}, {Block: failed.ID}}, Location: location(token)})
	}
	switch item := item.(type) {
	case *ast.SwitchValueCase:
		if !hasSubject {
			// §§ 12–13: a subjectless alternative is a bool condition.
			condition, err := fb.buildExpr(item.Value, 0)
			if err != nil {
				return err
			}
			if condition.typ != boolType {
				return fb.unsupported("non-boolean subjectless switch case", item.Token)
			}
			branch(condition, item.Token, body, next)
			return nil
		}
		value, err := fb.switchOperand(item.Value, subject, item.Token)
		if err != nil {
			return err
		}
		if fb.typeHasKind(subject.typ, TypeBool) {
			// § 27: a bool alternative matches when it equals the subject.
			whenTrue, whenFalse := fb.newBlock(), fb.newBlock()
			branch(subject, item.Token, whenTrue, whenFalse)
			fb.current = whenTrue
			branch(value, item.Token, body, next)
			fb.current = whenFalse
			branch(value, item.Token, next, body)
			return nil
		}
		if fb.typeHasKind(subject.typ, TypeEnum) {
			// § 24: an enum case alternative is same-enum value equality.
			equal := fb.result(Operation{Kind: OpEnumCompare, IntegerCompare: IntegerCompareEQ, Operands: []ValueID{subject.id, value.id}, Location: location(item.Token)}, boolType)
			branch(equal, item.Token, body, next)
			return nil
		}
		branch(fb.forCompare(IntegerCompareEQ, subject, value, boolType, location(item.Token)), item.Token, body, next)
		return nil
	case *ast.SwitchRelationalCase:
		predicate, ok := map[string]IntegerComparePredicate{"<": IntegerCompareLT, "<=": IntegerCompareLE, ">": IntegerCompareGT, ">=": IntegerCompareGE}[item.Operator]
		if !ok || !hasSubject || fb.typeHasKind(subject.typ, TypeEnum) || fb.typeHasKind(subject.typ, TypeBool) {
			return fb.unsupported("relational switch case "+item.Operator, item.Token)
		}
		value, err := fb.switchOperand(item.Value, subject, item.Token)
		if err != nil {
			return err
		}
		branch(fb.forCompare(predicate, subject, value, boolType, location(item.Token)), item.Token, body, next)
		return nil
	case *ast.SwitchRangeCase:
		if !hasSubject || item.Range == nil || item.Range.Start == nil && item.Range.End == nil || fb.typeHasKind(subject.typ, TypeEnum) || fb.typeHasKind(subject.typ, TypeBool) {
			return fb.unsupported("range switch case", item.Token)
		}
		loc := location(item.Token)
		// § 8: both bounds are evaluated, in order, before the membership test.
		var start, end builtValue
		var err error
		if item.Range.Start != nil {
			if start, err = fb.switchOperand(item.Range.Start, subject, item.Token); err != nil {
				return err
			}
		}
		if item.Range.End != nil {
			if end, err = fb.switchOperand(item.Range.End, subject, item.Token); err != nil {
				return err
			}
		}
		upper := func() builtValue {
			predicate := IntegerCompareLE
			if item.Range.Exclusive {
				predicate = IntegerCompareLT
			}
			return fb.forCompare(predicate, subject, end, boolType, loc)
		}
		switch {
		case item.Range.Start == nil:
			branch(upper(), item.Token, body, next)
		case item.Range.End == nil:
			branch(fb.forCompare(IntegerCompareGE, subject, start, boolType, loc), item.Token, body, next)
		default:
			checkUpper := fb.newBlock()
			branch(fb.forCompare(IntegerCompareGE, subject, start, boolType, loc), item.Token, checkUpper, next)
			fb.current = checkUpper
			branch(upper(), item.Token, body, next)
		}
		return nil
	}
	return fb.unsupported(fmt.Sprintf("switch case item %T", item), lexer.Token{})
}

// switchOperand builds a case operand in the subject's type.
func (fb *functionBuilder) switchOperand(expr ast.Expression, subject builtValue, token lexer.Token) (builtValue, error) {
	value, err := fb.buildExpr(expr, subject.typ)
	if err != nil {
		return builtValue{}, err
	}
	if value.typ != subject.typ {
		return builtValue{}, fb.unsupported("switch case of a different type", token)
	}
	return value, nil
}

func (fb *functionBuilder) typeHasKind(id TypeID, kind TypeKind) bool {
	typ, ok := fb.owner.module.Types.Lookup(id)
	return ok && typ.Kind == kind
}

func typeName(typ sema.Type) string {
	if typ.Name != "" {
		return typ.Name
	}
	return string(typ.Kind)
}
