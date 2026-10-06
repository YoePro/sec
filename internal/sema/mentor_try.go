package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// missingTryHelp explains the most likely fix when a Result or Option carrier
// is used where its success value was expected: the programmer forgot try.
// It returns "" when the carrier's success type would not fit either.
//
// Rules:
//   - rules/errors/errorhandling.md — §30 "Diagnostics must act as a mentor", §12 "Naked try propagation", §15
func missingTryHelp(expected Type, actual Type, expr ast.Expression) string {
	success, family, ok := carrierSuccessType(actual)
	if !ok || expected.Kind == InvalidType || !canInitialize(expected, success, nil) {
		return ""
	}
	return carrierTryHelp(actual, success, family, expr)
}

func carrierSuccessType(actual Type) (Type, string, bool) {
	switch {
	case actual.Kind == ResultType && len(actual.TypeArgs) == 2:
		return actual.TypeArgs[0], "Result", true
	case actual.Kind == UnionType && actual.Name == "Option" && len(actual.TypeArgs) == 1:
		return actual.TypeArgs[0], "Option", true
	}
	return Type{}, "", false
}

func carrierTryHelp(actual Type, success Type, family string, expr ast.Expression) string {
	spelling := "the expression"
	if expr != nil {
		spelling = expr.String()
	}
	if family == "Option" {
		return fmt.Sprintf("%s produces %s, not %s. Write `try %s` to use the %s and propagate None, or handle absence with `try %s { None => ... }`.",
			spelling, typeDisplayName(actual), typeDisplayName(success), spelling, typeDisplayName(success), spelling)
	}
	return fmt.Sprintf("%s produces %s, not %s. Write `try %s` to use the %s and propagate or handle %s, or use match to handle Ok and Err explicitly.",
		spelling, typeDisplayName(actual), typeDisplayName(success), spelling, typeDisplayName(success), typeDisplayName(actual.TypeArgs[1]))
}

// addTypeMismatchError reports an expected/actual mismatch and adds the
// missing-try explanation when it applies.
func (a *Analyzer) addTypeMismatchError(token lexer.Token, expected Type, actual Type, expr ast.Expression, format string, args ...any) {
	if suggestion, ok := a.unitConversionSuggestion(expected, actual, expr); ok {
		a.unitConversionSuggestions = append(a.unitConversionSuggestions, suggestion)
		a.addErrorAtTokenWithMetadata(token, "",
			"convert explicitly with `"+suggestion.Replacement+"`; "+a.implicitUnitConversionRejection(expected, actual, expr),
			format, args...)
		return
	}
	if help := missingTryHelp(expected, actual, expr); help != "" {
		a.addErrorAtTokenWithMetadata(token, "", help, format, args...)
		return
	}
	a.addErrorAtToken(token, format, args...)
}

// numericCarrierOperandHelp explains a Result or Option operand of an
// arithmetic operator whose success type is numeric.
func numericCarrierOperandHelp(left Type, right Type, leftExpr ast.Expression, rightExpr ast.Expression) string {
	for _, operand := range []struct {
		typ  Type
		expr ast.Expression
	}{{left, leftExpr}, {right, rightExpr}} {
		if success, family, ok := carrierSuccessType(operand.typ); ok && isNumericType(success) {
			return carrierTryHelp(operand.typ, success, family, operand.expr)
		}
	}
	return ""
}
