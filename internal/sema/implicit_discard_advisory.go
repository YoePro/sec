package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// recordImplicitDiscard records one legal standalone ordinary call result.
// Summary/backedge probes do not duplicate advice, and promotion is delayed
// until mandatory semantic analysis has completed so policy cannot alter flow.
// Rules: rules/control-flow/discard.md — §§10–13,37–38;
// rules/tooling/diagnostics.md — §18.
func (a *Analyzer) recordImplicitDiscard(stmt *ast.ExpressionStatement, typ Type) {
	if a.summaryPass || a.iterationDependencyProbe || !a.callGraphPathReachable {
		return
	}
	token := stmt.Token
	endLine, endColumn := token.EndPosition()
	fact := Error{ID: diagnostics.ImplicitDiscardedResult, Severity: diagnostics.SeverityInformation,
		Message: fmt.Sprintf("result of %s with type %s is implicitly discarded", expressionStatementDisplayName(stmt.Expression), typeDisplayName(typ)),
		Help:    "Use the result in a binding, return or enclosing expression, or write `discard` before the call to acknowledge that the result is intentionally ignored.",
		File:    token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn}
	var declaration lexer.Token
	switch expr := stmt.Expression.(type) {
	case *ast.CallExpression:
		if selected, found := a.ResolvedCallTarget(expr); found {
			declaration = selected.Function.Token
		}
	case *ast.NewExpression:
		if selected, found := a.ResolvedConstructionOf(expr); found {
			declaration = selected.Initializer.Token
		}
	}
	if declaration.Line > 0 {
		fact.PreviousFile, fact.PreviousLine, fact.PreviousColumn = declaration.File, declaration.Line, declaration.Column
		fact.RelatedLabel = "selected callable declaration"
	}
	a.implicitDiscards[stmt] = fact
}

// emitImplicitDiscardAdvisories publishes the deduplicated source occurrences
// through the ordinary project policy. Mandatory must-use diagnostics retain
// their own identities regardless of this advisory's severity or suppression.
// Rules: rules/control-flow/discard.md — §§12–13,38;
// rules/tooling/diagnostics.md — §18.
func (a *Analyzer) emitImplicitDiscardAdvisories() {
	facts := make([]Error, 0, len(a.implicitDiscards))
	for _, fact := range a.implicitDiscards {
		facts = append(facts, fact)
	}
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].File != facts[j].File {
			return facts[i].File < facts[j].File
		}
		if facts[i].Line != facts[j].Line {
			return facts[i].Line < facts[j].Line
		}
		return facts[i].Column < facts[j].Column
	})
	for _, fact := range facts {
		a.appendWarning(fact)
	}
}
