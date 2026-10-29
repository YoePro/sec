package sema

import (
	"fmt"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// provenArithmeticFailureVariants returns the ArithmeticError variants the
// protected subtree of a try can actually produce, when its complete failure
// provenance is known: every arithmetic failure point is a checked integer
// or decimal operation and no carrier can deliver an ArithmeticError of
// unknown origin.
// A Result-returning call typed Result[T, ArithmeticError] or Result[T,
// error] proves nothing about the variants its implementation produces.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — § 25A (failure provenance)
//   - rules/errors/errorhandling.md — § 27.2b
//   - rules/corrections/md029-md042-md102-pattern-reachability-some-correction-20261028.md — § 9
func (a *Analyzer) provenArithmeticFailureVariants(expr *ast.TryExpression) (map[string]bool, bool) {
	if rootType, ok := a.ResolvedTypeOf(expr.Expression); ok && rootType.Kind == ResultType && len(rootType.TypeArgs) == 2 && arithmeticErrorOpaque(rootType.TypeArgs[1]) {
		return nil, false
	}
	possible := map[string]bool{}
	for _, point := range a.collectTryFailurePoints(expr.Expression) {
		switch point.Kind {
		case TryFailureArithmetic:
			variants, ok := a.integerOperationFailureVariants(point.Expression)
			if !ok {
				return nil, false
			}
			for _, variant := range variants {
				possible[variant] = true
			}
		case TryFailureCarrier:
			if arithmeticErrorOpaque(point.ErrorType) {
				return nil, false
			}
		}
	}
	return possible, true
}

// arithmeticErrorOpaque reports a carrier error type that may deliver any
// ArithmeticError variant from an implementation the caller cannot inspect.
func arithmeticErrorOpaque(errorType Type) bool {
	return errorType.Kind == ErrorRootType || errorType.Name == "ArithmeticError"
}

// integerOperationFailureVariants lists the ArithmeticError variants of one
// checked integer operation (operators.md "Checked integer arithmetic",
// "Shift count") or decimal operation (its operator plan, MD-067 § 7(3));
// any other checked operation is not classified here.
func (a *Analyzer) integerOperationFailureVariants(expr ast.Expression) ([]string, bool) {
	operatorText := ""
	switch node := expr.(type) {
	case *ast.InfixExpression:
		operatorText = node.Operator
	case *ast.PrefixExpression:
		operatorText = "unary" + node.Operator
	default:
		return nil, false
	}
	operator, ok := a.ResolvedOperatorOf(expr)
	if ok && operator.Decimal != nil && operator.RuntimeCheck {
		// MD-067 § 7(3): the decimal operator plan lists its own variants.
		return operator.Decimal.PossibleFailures, true
	}
	if !ok || !isBuiltinIntegerOperatorType(dereferenceType(operator.LeftType)) {
		return nil, false
	}
	switch operatorText {
	case "+", "-", "*", "unary-":
		return []string{"Overflow"}, true
	case "/", "%":
		return []string{"DivisionByZero", "Overflow"}, true
	case "<<":
		return []string{"InvalidShift", "Overflow"}, true
	case ">>":
		return []string{"InvalidShift"}, true
	}
	return nil, false
}

// reportInactiveTryHandler explains a legally typed handler whose variant the
// complete failure provenance of the protected operations excludes.
// Rules: rules/control-flow/flowcontrol_match.md — § 25A; rules/tooling/diagnostics.md — § 24(4e).
func (a *Analyzer) reportInactiveTryHandler(handler *ast.TryHandler, errorType Type, variant string, possible map[string]bool) {
	names := make([]string, 0, len(possible))
	for name := range possible {
		names = append(names, typeDisplayName(errorType)+"."+name)
	}
	sort.Strings(names)
	a.addWarningAtTokenWithMetadata(handler.Token, diagnostics.InactiveMatchArm,
		fmt.Sprintf("The protected operations can fail only with %s. %s.%s remains a valid variant and this handler is legal.", strings.Join(names, ", "), typeDisplayName(errorType), variant),
		"this try handler cannot be selected: no protected operation can produce %s.%s", typeDisplayName(errorType), variant)
}
