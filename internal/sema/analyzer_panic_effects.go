package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// recordResolvedOperatorEffect publishes the exact set of canonical panic
// reasons possible for a checked integer operation. It consumes resolved
// operator meaning and types rather than rediscovering semantics from syntax.
//
// Rules:
//   - rules/errors/panic.md — §13(1)–(2) "Panic information and reason IDs"
//   - rules/errors/panic.md — §25 "Effect analysis"
//   - rules/errors/runtime_checks.md — "Arithmetic checks", "Division and remainder", "Shift checks"
func (a *Analyzer) recordResolvedOperatorEffect(expr ast.Expression) {
	if a.summaryPass || !a.callGraphPathReachable || expr == nil {
		return
	}
	resolved, ok := a.resolvedOperators[expr]
	if !ok || !resolved.RuntimeCheck || resolved.FailureBehavior != OperatorArithmeticFailure {
		return
	}
	reasons := arithmeticPanicReasonIDs(resolved)
	if len(reasons) == 0 {
		return
	}
	a.callGraph.addEffect(a.currentCallable, EffectSite{
		Kind:           EffectMayPanicArithmetic,
		Source:         expressionToken(expr),
		PanicReasonIDs: reasons,
	})
}

// arithmeticPanicReasonIDs maps one compiler-resolved integer operation to
// every panic category it can produce. Ordering follows the operation's stable
// runtime-check precedence, with invalid divisor/count checks before overflow.
func arithmeticPanicReasonIDs(resolved ResolvedOperator) []diagnostics.PanicReasonID {
	switch resolved.Kind {
	case ResolvedIntegerNegateChecked, ResolvedIntegerAddChecked,
		ResolvedIntegerSubtractChecked, ResolvedIntegerMultiplyChecked:
		return []diagnostics.PanicReasonID{diagnostics.PanicReasonArithmeticOverflow}
	case ResolvedIntegerDivideChecked, ResolvedIntegerRemainderChecked:
		reasons := []diagnostics.PanicReasonID{diagnostics.PanicReasonDivisionByZero}
		if resolved.LeftType.Kind == IntType {
			reasons = append(reasons, diagnostics.PanicReasonArithmeticOverflow)
		}
		return reasons
	case ResolvedIntegerShiftLeftSignedChecked:
		return []diagnostics.PanicReasonID{
			diagnostics.PanicReasonInvalidShift,
			diagnostics.PanicReasonArithmeticOverflow,
		}
	case ResolvedIntegerShiftLeftUnsignedChecked,
		ResolvedIntegerShiftRightUnsignedChecked,
		ResolvedIntegerShiftRightSignedChecked:
		return []diagnostics.PanicReasonID{diagnostics.PanicReasonInvalidShift}
	default:
		return nil
	}
}

// resolveArithmeticFailureEffect removes the ordinary panic edge after try has
// converted that exact operation's failure into typed ArithmeticError flow.
//
// Rules:
//   - rules/errors/runtime_checks.md — "Arithmetic checks"
//   - rules/errors/errorhandling.md — checked arithmetic with try
func (a *Analyzer) resolveArithmeticFailureEffect(expr ast.Expression) {
	if a.summaryPass || expr == nil {
		return
	}
	a.callGraph.removeEffect(a.currentCallable, EffectMayPanicArithmetic, expressionToken(expr))
}
