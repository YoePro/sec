package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// inferCompilerKnownFill resolves contextual fixed-array, owning-array and
// string construction. Array elements must support ordinary infallible copying,
// independently of the destination extent or requested runtime count.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill";
// rules/collections/collections.md — §§5,6,7.9;
// rules/memory/copy_move.md — §§3.1–3.4 copy classification.
func (a *Analyzer) inferCompilerKnownFill(expr *ast.CallExpression, expected Type, hasExpected bool) (Type, expressionValue, bool) {
	result := expressionValue{Display: expr.String()}
	if len(expr.GenericArguments) > 0 {
		a.addErrorAtToken(expr.Token, "fill uses target context and does not accept generic arguments")
		return Type{Kind: InvalidType}, result, true
	}
	if !hasExpected || expected.Kind == InvalidType || expected.Kind == "" {
		a.addErrorAtToken(expr.Token, "fill requires an explicit array or string target type")
		return Type{Kind: InvalidType}, result, true
	}
	if expected.Kind == ArrayType && expected.Element != nil {
		if arrayShapeOf(expected) == ArrayShapeFixed {
			if !a.checkCompilerKnownCallArity(expr, "fill", 1, 1) {
				return Type{Kind: InvalidType}, result, true
			}
			valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], *expected.Element)
			if !a.canInitialize(*expected.Element, valueType, expr.Arguments[0]) {
				a.addErrorAtToken(expressionToken(expr.Arguments[0]), "fill value must be %s, got %s", typeDisplayName(*expected.Element), typeDisplayName(valueType))
				return Type{Kind: InvalidType}, result, true
			}
			if a.rejectNonCopyableFillElement(*expected.Element, expr.Token, "fill") {
				return Type{Kind: InvalidType}, result, true
			}
			return expected, result, true
		}
		if !a.checkCompilerKnownCallArity(expr, "fill", 2, 2) {
			return Type{Kind: InvalidType}, result, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], *expected.Element)
		if !a.canInitialize(*expected.Element, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "fill value must be %s, got %s", typeDisplayName(*expected.Element), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, result, true
		}
		if a.rejectNonCopyableFillElement(*expected.Element, expr.Token, "fill") {
			return Type{Kind: InvalidType}, result, true
		}
		if !a.compilerKnownCountArgument(expr.Arguments[1], "fill") {
			return Type{Kind: InvalidType}, result, true
		}
		return compilerKnownResult(expected, a.types["CollectionError"]), result, true
	}
	if expected.Kind == StringType {
		if !a.checkCompilerKnownCallArity(expr, "fill", 2, 2) {
			return Type{Kind: InvalidType}, result, true
		}
		fragmentType, _ := a.inferExpressionWithExpected(expr.Arguments[0], expected)
		if fragmentType.Kind != StringType && fragmentType.Kind != CharType && fragmentType.Kind != RuneType {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "string fill fragment must be string, char, or rune, got %s", typeDisplayName(fragmentType))
			return Type{Kind: InvalidType}, result, true
		}
		if !a.compilerKnownCountArgument(expr.Arguments[1], "fill") {
			return Type{Kind: InvalidType}, result, true
		}
		return compilerKnownResult(expected, a.types["CollectionError"]), result, true
	}
	a.addErrorAtToken(expr.Token, "fill target must be a fixed array, owning dynamic array, or string, got %s", typeDisplayName(expected))
	return Type{Kind: InvalidType}, result, true
}

// rejectNonCopyableFillElement enforces repeated-copy availability for both
// contextual construction and mutable-slice replacement.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill";
// rules/collections/collections.md — §7.9 "Slice Fill";
// rules/memory/copy_move.md — §§3.1–3.4 copy classification.
func (a *Analyzer) rejectNonCopyableFillElement(element Type, token lexer.Token, operation string) bool {
	classification := CopyClassificationOf(element)
	if classification == CopyTrivial || classification == CopySemantic {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.FillElementNotCopyable,
		"fill evaluates its source once and copies it into each element. Construct move-only elements individually; a zero extent or count does not make this copy operation available.",
		"%s requires a copyable element type, got %s", operation, typeDisplayName(element))
	return true
}
