package sema

import "sec/internal/ast"

// inferStringFromUtf8Intrinsic types the internal string-materialization
// capability `_stringFromUtf8`. Its operand is any borrowed contiguous byte
// sequence — `ref byte[]`, `ref byte[N]` or a byte slice — so core formatting
// helpers with fixed local buffers can use it. The
// operation validates strict UTF-8 itself and returns
// Result[string, StringError]; it never consumes, adopts or mutates the
// borrowed bytes. Core-only authority is checked by the caller.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "Internal core string materialization" (MD-091)
//   - rules/library/core-library.md — § 5.2
func (a *Analyzer) inferStringFromUtf8Intrinsic(expr *ast.CallExpression, known CompilerKnownFunction) (Type, expressionValue, bool) {
	result := expressionValue{Display: expr.String()}
	argument := expr.Arguments[0]
	actual, _ := a.inferExpression(argument)
	if actual.Kind == InvalidType {
		return Type{Kind: InvalidType}, result, true
	}
	borrowed := actual.Kind == ReferenceType || actual.Kind == SliceType
	sequence := dereferenceType(actual)
	byteSequence := (sequence.Kind == ArrayType || sequence.Kind == SliceType) && sequence.Element != nil && sequence.Element.Kind != InvalidType && sequence.Element.Name == "byte"
	if !borrowed || !byteSequence {
		a.addErrorAtToken(expressionToken(argument), "argument 1 to %s must be a borrowed byte sequence (ref byte[], ref byte[N] or a byte slice), got %s", known.Name, typeDisplayName(actual))
		return Type{Kind: InvalidType}, result, true
	}
	return a.refinedCompilerKnownType(known.Result), result, true
}
