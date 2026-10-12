package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// isNullSentinel reports the reserved source sentinel `null`. null is a hard
// keyword (MD-024 §5.2), so no binding can shadow it.
func (a *Analyzer) isNullSentinel(expr ast.Expression) bool {
	identifier, ok := expr.(*ast.Identifier)
	return ok && identifier.Value == "null" && len(identifier.ForeignSeparators) == 0
}

// inferNullSentinel types the raw-pointer sentinel. null is not an ordinary
// Sec value: it is legal only lexically inside unsafe, has no standalone
// type, and takes its RawPtr[T] type from the target context.
//
// Rules:
//   - rules/platform/ffi.md — §11 "null"
//   - rules/memory/raw_pointers.md — § 7(9) the sentinel syntax is restricted by the FFI rules
func (a *Analyzer) inferNullSentinel(expr *ast.Identifier) (Type, expressionValue) {
	if !a.inUnsafe {
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.NullOutsideUnsafe,
			"Move the operation into an unsafe { } block whose proof obligations you accept.",
			"null is an unsafe raw-pointer sentinel and may be used only inside an unsafe context")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	expected, ok := a.expectedExpressionTypes[expr]
	if ok && expected.Kind == RawPtrType {
		return expected, expressionValue{Display: expr.String()}
	}
	if ok && isOptionType(expected) {
		// MD-024 §5.6: Option absence is None, never the foreign sentinel.
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.NullWithoutRawPointerContext,
			"Write None for an absent Option value; null exists only for RawPtr[T] at unsafe foreign boundaries.",
			"null is not an Option value; use None")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.NullWithoutRawPointerContext,
		"Give the target an explicit RawPtr[T] type, for example let value: RawPtr[Device] := null.",
		"null has no standalone type; it needs a RawPtr[T] target context")
	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

// inferNullTestExpression validates `subject is null` and `subject is not
// null`: unsafe-only tests on a RawPtr[T] subject that yield bool in any
// boolean-expression position without dereferencing the pointer.
//
// Rules:
//   - rules/platform/ffi.md — §11 "Null testing uses is"; "Invalid" outside unsafe
//   - rules/memory/raw_pointers.md — § 7(3) testing a null raw pointer is permitted
//   - rules/corrections/applied/md020-md021-md023-md024-ffi-c-bindings-null-correction-20261008.md — §5.3–5.5
func (a *Analyzer) inferNullTestExpression(expr *ast.NullTestExpression) (Type, expressionValue) {
	boolType := Type{Name: "bool", Kind: BoolType}
	test := "is null"
	if expr.Negated {
		test = "is not null"
	}
	if !a.inUnsafe {
		a.addErrorAtTokenWithMetadata(expr.NullToken, diagnostics.NullOutsideUnsafe,
			"Move the test into an unsafe { } block.",
			"%s uses the unsafe raw-pointer sentinel and may be written only inside an unsafe context", test)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	subject, _ := a.inferExpression(expr.Subject)
	if subject.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if subject.Kind != RawPtrType {
		a.addErrorAtTokenWithMetadata(expressionToken(expr.Subject), diagnostics.NullTestRequiresRawPointer,
			"Only RawPtr[T] values can be null; safe references and other values are never null.",
			"%s requires a RawPtr[T] subject, got %s", test, typeDisplayName(subject))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	return boolType, expressionValue{Display: expr.String()}
}

// rejectNullEquality reports `== null` and `!= null`, which ffi.md §11
// forbids in every context in favor of `is null`.
func (a *Analyzer) rejectNullEquality(expr *ast.InfixExpression) bool {
	if expr.Operator != "==" && expr.Operator != "!=" {
		return false
	}
	if !a.isNullSentinel(expr.Left) && !a.isNullSentinel(expr.Right) {
		return false
	}
	a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.NullEquality,
		"Test the raw pointer with is null inside unsafe instead.",
		"equality comparison with null is invalid; use is null")
	return true
}

// bindNullArgumentContexts gives a contextual argument its target type when
// every candidate callable with a parameter in that position agrees on the
// same type of the required family: RawPtr[T] for the null sentinel and
// Option[T] for a bare None. Disagreeing or other candidates provide no
// context, so the argument keeps no standalone type there. The returned
// function removes the temporary contexts.
//
// Rules:
//   - rules/platform/ffi.md — §11 "A target type may provide the required raw-pointer context"
//   - rules/foundations/language_philosophy.md — §16 "Infer, do not guess"
//   - rules/types/types.md — "Contextual None and empty-literal arguments" (MD-027)
func (a *Analyzer) bindNullArgumentContexts(functions []Function, arguments []ast.Expression) func() {
	bound := []ast.Expression{}
	for index, argument := range arguments {
		var family func(Type) bool
		switch {
		case a.isNullSentinel(argument):
			family = func(typ Type) bool { return typ.Kind == RawPtrType }
		case a.isBareNoneArgument(argument):
			family = isOptionType
		case isEmptyArrayLiteral(argument):
			family = acceptsEmptyArrayLiteral
		default:
			continue
		}
		var context *Type
		agreed := true
		for _, function := range functions {
			// Only candidates that can accept this argument count compete.
			if index >= len(function.Parameters) || !functionAcceptsCallArguments(function, len(arguments), make([]bool, len(arguments))) {
				continue
			}
			parameter := function.Parameters[index].Type
			if !family(parameter) || context != nil && !sameConcreteType(*context, parameter) {
				agreed = false
				break
			}
			context = &parameter
		}
		if !agreed || context == nil {
			continue
		}
		if _, exists := a.expectedExpressionTypes[argument]; exists {
			continue
		}
		a.expectedExpressionTypes[argument] = *context
		bound = append(bound, argument)
	}
	return func() {
		for _, argument := range bound {
			delete(a.expectedExpressionTypes, argument)
		}
	}
}

// isBareNoneArgument reports an unqualified None that names the Option
// variant rather than a visible binding.
func (a *Analyzer) isBareNoneArgument(expr ast.Expression) bool {
	identifier, ok := expr.(*ast.Identifier)
	if !ok || identifier.Value != "None" {
		return false
	}
	_, shadowed := a.symbols["None"]
	return !shadowed
}

func isOptionType(typ Type) bool {
	return typ.Kind == UnionType && typ.Name == "Option"
}
