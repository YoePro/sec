package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// nonEmptyAcrossScalarTraversal retains an immediately preceding exit proof
// across one canonical shortened loop whose only operations read and discard
// scalar elements. No call, write, borrow, getter, cleanup-bearing value or
// nested execution is accepted without the corresponding stability facts.
// This deliberately does not claim general mutation-aware dominance.
// Rules: rules/analysis/pitfall_analysis.md — "Final-element access requires non-empty proof",
// "Guards participate in pitfall reasoning", "False-positive regression corpus";
// rules/control-flow/flowcontrol_for.md — §§23, 25, 39.
func (b *pitfallBuilder) nonEmptyAcrossScalarTraversal(collection string, receiver ast.Expression) (lexer.Token, bool) {
	preceding := b.activePreceding
	if len(preceding) < 2 || !b.pitfallPureOperand(receiver) {
		return lexer.Token{}, false
	}
	loop, ok := preceding[len(preceding)-1].(*ast.ForStatement)
	if !ok || loop.Body == nil || loop.Step != nil || len(loop.Bindings) != 1 || loop.Bindings[0].Discard {
		return lexer.Token{}, false
	}
	domain, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || !domain.Exclusive || !b.pitfallIntegerEquals(domain.Start, 0) {
		return lexer.Token{}, false
	}
	endpoint, ok := domain.End.(*ast.InfixExpression)
	if !ok || endpoint.Operator != "-" || !b.pitfallIntegerEquals(endpoint.Right, 1) {
		return lexer.Token{}, false
	}
	boundCollection, _, resolved := b.lengthReceiver(endpoint.Left)
	length, member := endpoint.Left.(*ast.MemberExpression)
	if !resolved || boundCollection != collection || !member || !b.pitfallPureOperand(length.Object) {
		return lexer.Token{}, false
	}
	guarded, proof, valid := b.emptyCollectionExitGuard(preceding[len(preceding)-2])
	if !valid || guarded != collection {
		return lexer.Token{}, false
	}
	for _, statement := range loop.Body.Statements {
		discard, ok := statement.(*ast.DiscardStatement)
		if !ok {
			return lexer.Token{}, false
		}
		index, ok := discard.Value.(*ast.IndexExpression)
		if !ok || !b.pitfallPureOperand(index.Left) || !b.expressionUsesBinding(index.Index, loop.Bindings[0].Token) {
			return lexer.Token{}, false
		}
		indexed, resolved := b.expressionIdentity(index.Left)
		typ, typed := b.analyzer.ResolvedTypeOf(index)
		if !resolved || indexed != collection || !typed || typ.CustomFree || !(isIntegerType(typ) || typ.Kind == BoolType || typ.Kind == CharType || typ.Kind == RuneType) {
			return lexer.Token{}, false
		}
	}
	return proof, true
}
