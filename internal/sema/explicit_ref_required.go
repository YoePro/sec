package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// rejectImplicitBorrow reports an owned value used where a direct reference
// is required outside a call argument. Marker-free borrowing exists only at
// call sites (borrowing.md § 15.2); every other reference creation needs an
// explicit `ref` / `ref mut` (references.md § 5). Accepting the owned value
// would create a reference without a recorded origin, so a borrow of local
// storage stored through a `ref mut` parameter or returned from the function
// would escape unchecked and dangle. Slice views keep their own conversion
// rules and are not handled here.
//
// Rules:
//   - rules/memory/borrowing.md — §§ 5.1, 15.2
//   - rules/memory/references.md — § 5
func (a *Analyzer) rejectImplicitBorrow(target, value Type, expr ast.Expression) bool {
	if expr == nil || target.Kind != ReferenceType || target.Element == nil || target.Element.Kind == SliceType {
		return false
	}
	switch value.Kind {
	case ReferenceType, InvalidType, NeverType, AnyType:
		return false
	}
	if !canInitialize(*target.Element, value, expr) {
		return false
	}
	marker := "ref"
	if target.ReferenceMutable {
		marker = "ref mut"
	}
	a.addErrorAtTokenWithMetadata(expressionToken(expr), diagnostics.BorrowExplicitRefRequired,
		fmt.Sprintf("Only a call argument borrows implicitly. Write `%s %s` to borrow explicitly; the borrow is then checked like any other, so a reference to local storage cannot be returned or stored beyond its lifetime.", marker, expr.String()),
		"%s is required here, got owned %s; write `%s %s`", typeDisplayName(target), typeDisplayName(value), marker, expr.String())
	return true
}
