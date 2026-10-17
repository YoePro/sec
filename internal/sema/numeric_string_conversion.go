package sema

import "sec/internal/ast"

// numericStringConversionType types the explicit conversion
// string(numericValue) (MD-051, 2026-10-17): it is the no-argument numeric
// ToString() under another spelling, so its result is exactly
// Result[string, StringError] with the same default formatting, allocation
// and failure contract. It is never a code-point conversion and never an
// implicit coercion; char and rune conversions are unaffected.
//
// Rules:
//   - rules/library/core-library.md — § 15 "ToString and formatting"
//   - rules/types/types.md — "Explicit conversions"
//   - rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md — § 7
func (a *Analyzer) numericStringConversionType(expr ast.Expression, target, value Type) (Type, bool) {
	if target.Kind != StringType || target.Named || !isNumericType(value) {
		return Type{}, false
	}
	result := toStringResultType()
	// Use the trusted-core StringError declaration when it is loaded, so the
	// result is the same type the numeric ToString() declarations return.
	if failure, declared := a.types["StringError"]; declared && failure.Kind == UnionType {
		result = compilerKnownResult(builtinType("string"), failure)
	}
	a.expressionTypes[expr] = result
	return result, true
}
