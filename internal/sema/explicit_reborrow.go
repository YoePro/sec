package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// reportExplicitReborrow emits the educational Information diagnostic of a
// legal explicit reborrow (MD-061 § 5): the expression refers to the holder's
// referent, not to the holder's storage. It deliberately does not suggest
// borrowing the owner directly, which may be unavailable while the holder has
// exclusive authority.
//
// Rules:
//   - rules/memory/borrowing.md — § 11.4 "Explicit reborrow"
//   - rules/tooling/diagnostics.md — § 4(4), § 18
func (a *Analyzer) reportExplicitReborrow(expr *ast.RefExpression, referent string, element Type) {
	holder := expr.Value.String()
	kind := "ref"
	if expr.Mutable {
		kind = "ref mut"
	}
	target := referent
	if target == "" || target == holder || strings.HasPrefix(target, "$") {
		target = "its referent"
	} else {
		target = "'" + target + "'"
	}
	a.addWarningAtTokenWithMetadata(expr.Token, diagnostics.ExplicitReborrow,
		"The expression refers to "+target+", not to the storage of '"+holder+"'.",
		"reborrow of %s through reference holder '%s'; result type %s %s", target, holder, kind, typeDisplayName(element))
}
