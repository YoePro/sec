package sema

import (
	"strings"

	"sec/internal/ast"
)

// explicitSliceHelp names the explicit slice borrow when a slice is expected
// and the value is a sequence of the same element type that could be
// borrowed as one: T[N], T[] or a reference to either. Sec 0.1 has no
// implicit owner- or reference-to-slice conversion, so the help only spells
// the existing explicit form (MD-149 tracks the ergonomics question).
// Rules: rules/collections/collections.md — §§ 7.1-7.3; rules/memory/reference_model.md — § 14.
func explicitSliceHelp(expected, actual Type, expr ast.Expression) string {
	if expected.Kind != ReferenceType || expected.Element == nil || expected.Element.Kind != SliceType || expected.Element.Element == nil {
		return ""
	}
	source, borrowed := actual, false
	if source.Kind == ReferenceType && source.Element != nil {
		source, borrowed = *source.Element, true
		if source.Kind == SliceType {
			return ""
		}
	}
	if source.Kind != ArrayType || source.Element == nil || !sameConcreteType(*source.Element, *expected.Element.Element) {
		return ""
	}
	operand := "values"
	if expr != nil {
		operand = expr.String()
		if borrowed {
			if prefix, ok := expr.(*ast.RefExpression); ok && prefix.Value != nil {
				operand = prefix.Value.String()
			} else {
				operand = strings.TrimPrefix(strings.TrimPrefix(operand, "ref mut "), "ref ")
			}
		}
	}
	borrow := "ref "
	if expected.ReferenceMutable {
		borrow = "ref mut "
	}
	return "borrow a slice explicitly with `" + borrow + operand + "[..]` (or a subrange such as `" + borrow + operand + "[0..<n]`); Sec has no implicit array-to-slice conversion"
}
