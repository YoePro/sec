package sema

import "sec/internal/ast"

// selectableOperationType resolves eligible wait operations and preserves the
// explicit prohibition on synthesizing observer completion waits.
// Rules: rules/concurrency/select.md — selectable operations;
// rules/concurrency/threads.md — §43(1–4).
func (a *Analyzer) selectableOperationType(expr ast.Expression) (Type, bool) {
	switch expr := expr.(type) {
	case *ast.AwaitExpression:
		typ, _ := a.inferAwaitExpression(expr)
		return typ, true
	case *ast.CallExpression:
		member, ok := expr.Callee.(*ast.MemberExpression)
		if !ok || member.Property == nil {
			a.inferExpression(expr)
			return Type{Kind: InvalidType}, false
		}
		receiverType, ok := a.compilerKnownReceiverType(member.Object)
		receiverType = dereferenceType(receiverType)
		if !ok {
			a.inferExpression(expr)
			return Type{Kind: InvalidType}, false
		}
		if isProcessObserverType(receiverType) && member.Property.Value == "Wait" {
			typ, _ := a.inferExpression(expr)
			return typ, true
		}
		if isThreadObserverType(receiverType) {
			a.reportThreadObserverWait(member.Property.Token)
			return Type{Kind: InvalidType}, true
		}
		// select.md § 54(3): TrySend/TryReceive never wait, so they are
		// ordinary calls, not selectable operations.
		if isReceiverType(receiverType) {
			switch member.Property.Value {
			case "Receive":
				typ, _ := a.inferExpression(expr)
				return typ, true
			}
		}
		if isSenderType(receiverType) {
			switch member.Property.Value {
			case "Send", "SendRevocable":
				typ, _ := a.inferExpression(expr)
				return typ, true
			}
		}
		a.inferExpression(expr)
		return Type{Kind: InvalidType}, false
	default:
		typ, _ := a.inferExpression(expr)
		if isThreadObserverType(typ) {
			a.reportThreadObserverWait(expressionToken(expr))
			return Type{Kind: InvalidType}, true
		}
		return Type{Kind: InvalidType}, false
	}
}
