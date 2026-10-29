package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// reportImplicitEmptyCatchAll uses the match entry's existing empty-state
// reachability fact. It advises only at an unguarded underscore, never a
// guarded fallback, initialized value, or match with an explicit empty arm.
// The advisory does not alter coverage or initialization/ownership flow.
// Rules: rules/declarations/unions.md — §§10.2–10.4,11;
// rules/tooling/diagnostics.md — §18 "Configuration and suppression".
func (a *Analyzer) reportImplicitEmptyCatchAll(expr *ast.MatchExpression, maybeEmptySubject string, accepted []ResolvedMatchArm) {
	if maybeEmptySubject == "" {
		return
	}
	for _, arm := range expr.Arms {
		if arm != nil && arm.Pattern != nil && arm.Pattern.Kind == ast.MatchPatternEmpty {
			return
		}
	}
	for _, resolved := range accepted {
		if resolved.SourceIndex < 0 || resolved.SourceIndex >= len(expr.Arms) {
			continue
		}
		arm := expr.Arms[resolved.SourceIndex]
		if arm == nil || arm.Pattern == nil || arm.Guard != nil || arm.Pattern.Kind != ast.MatchPatternCatchAll {
			continue
		}
		a.addWarningAtTokenWithMetadata(arm.Pattern.Token, diagnostics.ImplicitEmptyCatchAll,
			"Add an explicit `empty => ...` arm to distinguish uninitialized state from declared variants. Configure `union.implicit-empty-catch-all` if this fallback is intentional.",
			"catch-all pattern also handles the possible empty state of %s", maybeEmptySubject)
		return
	}
}
