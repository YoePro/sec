package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// rejectNonMemberStaticFunction rejects static functions registered as ordinary
// declarations. An enclosing impl method does not make a local declaration a
// member. Direct impl members use named registration; interface requirements
// have their own declaration path and remain receiver-free contracts.
// Rules: rules/declarations/static.md — §§3,9;
// rules/declarations/functions.md — §§22–24;
// rules/foundations/grammar.md — "Static declarations", "Statement grammar".
func (a *Analyzer) rejectNonMemberStaticFunction(fn *ast.FunctionDeclaration) bool {
	if !fn.Static {
		return false
	}
	a.addErrorAtTokenWithMetadata(fn.Token, diagnostics.StaticFunctionContext,
		"Declare a static function directly as an impl member (or an interface requirement). Use an ordinary fn for a module function; use a lambda for a local callable.",
		"static fn is only valid inside impl; use fn for a module function")
	return true
}
