package semantic

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/sema"
	"strings"
)

// buildCompoundAssignment lowers `local op= value` for an integer mutable
// local: the destination is read once, the source evaluated once, the
// Sema-resolved underlying operator applied with its checks, and the result
// written once only after the checks pass. Other targets remain an explicit
// boundary.
//
// Rules:
//   - rules/foundations/operators.md — "Compound assignment", "Compound arithmetic failure", "Increment and decrement aliases"
//   - rules/compiler/semantic_ir.md — § 31 "Storage operations", § 58 "Arithmetic"
func (fb *functionBuilder) buildCompoundAssignment(stmt *ast.AssignmentStatement) error {
	resolved, ok := fb.owner.analyzer.ResolvedCompoundAssignmentOf(stmt)
	if !ok || fb.owner.maxPackage < 13 {
		return fb.unsupported("compound assignment "+stmt.Operator, stmt.Token)
	}
	id, ok := stmt.Target.(*ast.Identifier)
	if !ok {
		return fb.unsupported("compound assignment to a non-local place", stmt.Token)
	}
	if _, ok := stmt.Value.(*ast.TryExpression); ok {
		return fb.unsupported("try arithmetic", expressionToken(stmt.Value))
	}
	if static, isStatic, err := fb.staticOfIdentifier(id); isStatic {
		if err != nil {
			return err
		}
		return fb.buildStaticCompoundAssignment(stmt, resolved, static)
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(id)
	if !ok {
		return fmt.Errorf("missing resolved assignment binding")
	}
	bind, ok := fb.bindings[fact.ID]
	if !ok || bind.storage == 0 {
		return fmt.Errorf("assignment target has no semantic storage")
	}
	leftType, err := fb.owner.internType(resolved.LeftType)
	if err != nil {
		return err
	}
	if leftType != bind.typ {
		return fmt.Errorf("compound assignment type does not match its storage")
	}
	loc := location(stmt.Token)
	current := fb.result(Operation{Kind: OpStorageLoad, Storage: bind.storage, Location: loc}, bind.typ)
	updated, err := fb.buildCompoundValue(stmt, resolved, current)
	if err != nil {
		return err
	}
	fb.emit(Operation{Kind: OpStorageStore, Storage: bind.storage, Operands: []ValueID{updated.id}, Location: loc})
	return nil
}

// buildCompoundValue consumes an already evaluated/read destination and Sema's
// integer operator plan. It evaluates the RHS once and returns only on the
// arithmetic-success path; the caller commits to the original Place/storage.
// Rules: rules/foundations/operators.md — "Compound assignment", "Compound arithmetic failure";
// rules/compiler/semantic_ir.md — §58 "Arithmetic".
func (fb *functionBuilder) buildCompoundValue(stmt *ast.AssignmentStatement, resolved sema.ResolvedOperator, current builtValue) (builtValue, error) {
	leftType, err := fb.owner.internType(resolved.LeftType)
	if err != nil {
		return builtValue{}, err
	}
	if resolved.RightType == nil {
		return builtValue{}, fmt.Errorf("compound assignment lacks a right operand type")
	}
	rightType, err := fb.owner.internType(*resolved.RightType)
	if err != nil {
		return builtValue{}, err
	}
	resultType, err := fb.owner.internType(resolved.ResultType)
	if err != nil {
		return builtValue{}, err
	}
	if current.typ != leftType || current.typ != resultType {
		return builtValue{}, fmt.Errorf("compound assignment type does not match its destination")
	}
	value, err := fb.buildExpr(stmt.Value, rightType)
	if err != nil {
		return builtValue{}, err
	}
	op := Operation{Operands: []ValueID{current.id, value.id}, Location: location(stmt.Token), Operator: strings.TrimSuffix(stmt.Operator, "=")}
	if !semanticOperatorKind(resolved, &op) {
		return builtValue{}, fb.unsupported("compound assignment "+stmt.Operator, stmt.Token)
	}
	if resolved.RuntimeCheck || resolved.ShiftCountProven {
		return fb.emitCheckedOperator(op, resultType, nil, nil, nil)
	}
	return fb.result(op, resultType), nil
}
