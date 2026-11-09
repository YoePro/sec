package sema

import "sec/internal/ast"

// analyzeJoinStatement checks `join worker`: the operand is an owned,
// available Task[T] or Thread[T] binding; join waits for terminal completion,
// consumes the one-shot join capability and preserves the handle, so the
// lifecycle is resolved for scope exit (closedResources, joined per path) and
// a second join is invalid. Join does not produce or consume the result; a
// task's TaskOutcome[T] stays with await.
//
// Rules:
//   - rules/concurrency/threads.md — § 34 "Join semantics"
//   - rules/concurrency/tasks.md — § 17 "Join"
//   - rules/foundations/grammar.md — § 61A "Join statement"
func (a *Analyzer) analyzeJoinStatement(stmt *ast.JoinStatement) {
	if stmt == nil || stmt.Value == nil {
		return
	}
	valueType, _ := a.inferExpression(stmt.Value)
	if valueType.Kind == InvalidType {
		return
	}
	if !isThreadType(valueType) && !isTaskType(valueType) {
		help := "join waits for a spawned Task[T] or Thread[T] handle."
		a.addErrorAtTokenWithMetadata(expressionToken(stmt.Value), "", help, "join requires Task[T] or Thread[T], got %s", typeDisplayName(valueType))
		return
	}
	identifier, ok := stmt.Value.(*ast.Identifier)
	if !ok {
		a.addErrorAtTokenWithMetadata(expressionToken(stmt.Value), "", "Bind the thread handle to a local first: the join capability belongs to one owned handle.", "join requires an owned thread binding")
		return
	}
	symbol, exists := a.symbols.lookup(identifier.Value)
	if !exists || symbol.Type.Kind == ReferenceType {
		a.addErrorAtTokenWithMetadata(identifier.Token, "", "A reference does not own the thread's join capability; join the owning binding.", "join requires an owned thread binding, got %s", typeDisplayName(symbol.Type))
		return
	}
	if previous, joined := a.closedResources[identifier.Value]; joined {
		kind := "thread"
		if isTaskType(valueType) {
			kind = "task"
		}
		a.addErrorAtTokenWithMetadata(identifier.Token, "",
			"join consumes the one-shot join capability; the handle stays usable for terminal inspection.",
			"%s %s was already joined at %d:%d", kind, identifier.Value, previous.Line, previous.Column)
		return
	}
	a.closedResources[identifier.Value] = stmt.Token
}
