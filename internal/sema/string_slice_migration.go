package sema

import (
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// rejectRetiredStringSlice reports the removed public member without guessing
// a semantics-preserving rewrite: brackets return a borrow, not owned text.
// An explicitly declared method on a distinct named type remains its own API.
// Rules: rules/compiler/compiler_known_members.md §21(3)-(5);
// rules/library/core-library.md §5.8; MD-126 §4(8a)-(10).
func (a *Analyzer) rejectRetiredStringSlice(owner Type, name string, token lexer.Token) bool {
	owner = dereferenceType(owner)
	if name != "Slice" || owner.Kind != StringType {
		return false
	}
	if owner.Named && owner.Name != "string" && len(a.functions[owner.Name+".Slice"]) > 0 {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.UnknownCallTarget,
		"Use text[a..<b], or try text[a..<b] for recoverable RangeError. The result is a borrowed StringView tied to the source lifetime, not an owned string. Materialize text explicitly with ToString() when an independent string is needed; handle its StringError. No automatic rewrite is applied.",
		"string.Slice has been removed; use a borrowed string range")
	return true
}
