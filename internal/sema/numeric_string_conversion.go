package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// reportUnavailableConversion reports a missing relation independently of
// checked-conversion failure. A named string receives explicit materialization
// guidance rather than an implicit numeric formatting step.
// Rules: rules/types/types.md — "Explicit conversions", "Numeric-to-text conversion";
// rules/tooling/diagnostics.md — § 2(6), § 3(6).
func (a *Analyzer) reportUnavailableConversion(token lexer.Token, target, value Type) {
	help := "Use a source value for which the target type defines an explicit conversion; try cannot create a missing conversion relation."
	if target.Kind == StringType && target.Named && isNumericType(value) {
		help = fmt.Sprintf("Materialize text explicitly: use `try %s(number.ToString())`, or `%s(try number.ToString())` when the outer conversion is infallible. Handle StringError and any independent target contract failure.", typeDisplayName(target), typeDisplayName(target))
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.ConversionUnavailable, help,
		"cannot convert %s to %s", typeDisplayName(value), typeDisplayName(target))
}

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
