package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// analyzePanicStatement records canonical explicit-panic metadata and, on a
// reachable live path, the matching non-returning panic effect. The parser has
// already restricted the optional message to one static string literal.
//
// Rules:
//   - rules/errors/panic.md — § 13 "Panic information and reason IDs"
//   - rules/errors/panic.md — § 17 "Explicit panic"
//   - rules/errors/panic.md — § 21 "@noPanic"
func (a *Analyzer) analyzePanicStatement(stmt *ast.PanicStatement) {
	if stmt == nil {
		return
	}
	message := ""
	hasMessage := stmt.Message != nil
	if hasMessage {
		message = stmt.Message.Value
	}
	a.resolvedExplicitPanics[stmt] = ResolvedExplicitPanic{
		Reason:     PanicReasonExplicitPanic,
		ReasonID:   diagnostics.PanicReasonExplicitPanic,
		Message:    message,
		HasMessage: hasMessage,
		File:       stmt.Token.File,
		Line:       stmt.Token.Line,
		Column:     stmt.Token.Column,
		Function:   a.currentFunctionName,
	}
	if a.summaryPass || !a.callGraphPathReachable {
		return
	}
	a.callGraph.addEffect(a.currentCallable, EffectSite{
		Kind:           EffectMayPanicExplicit,
		Source:         stmt.Token,
		PanicReasonIDs: []diagnostics.PanicReasonID{diagnostics.PanicReasonExplicitPanic},
	})
}

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

// recordSequenceIndexEffect publishes the ordinary bounds check of a
// dynamic-array or slice index, whose length is run-time state, as a
// may-panic effect. A try that converts the check removes it again.
//
// Rules:
//   - rules/errors/runtime_checks.md — "Index checks"
//   - rules/errors/panic.md — BoundsFailure, § 21 "@noPanic"
func (a *Analyzer) recordSequenceIndexEffect(expr *ast.IndexExpression) {
	if a.summaryPass || expr == nil {
		return
	}
	source := expressionToken(expr)
	a.callGraph.removeEffect(a.currentCallable, EffectMayPanicBounds, source)
	if !a.callGraphPathReachable {
		return
	}
	a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicBounds, Source: source, PanicReasonIDs: []diagnostics.PanicReasonID{diagnostics.PanicReasonBoundsFailure}})
}

// recordContractConversionEffect publishes an ordinary run-time conversion
// into a constrained named type as a may-panic contract effect.
//
// Rules:
//   - rules/types/contracts.md — "Runtime conversion into a constrained named type is fallible"
//   - rules/errors/panic.md — ContractFailure, § 21 "@noPanic"
func (a *Analyzer) recordContractConversionEffect(call *ast.CallExpression) {
	if a.summaryPass || call == nil {
		return
	}
	a.callGraph.removeEffect(a.currentCallable, EffectMayPanicContract, call.Token)
	if !a.callGraphPathReachable {
		return
	}
	a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicContract, Source: call.Token, PanicReasonIDs: []diagnostics.PanicReasonID{diagnostics.PanicReasonContractFailure}})
}

// resolveContractFailureEffect removes the contract panic of a conversion
// whose failure a try or fallible assignment converts into ContractError.
func (a *Analyzer) resolveContractFailureEffect(call *ast.CallExpression) {
	if a.summaryPass || call == nil {
		return
	}
	a.callGraph.removeEffect(a.currentCallable, EffectMayPanicContract, call.Token)
}

// recordForeignAbortEffect treats a call to an extern function as possibly
// aborting unless its declaration carries a trusted @noPanic foreign
// contract; unknown foreign behavior is never positive noPanic proof.
//
// Rules:
//   - rules/errors/panic.md — § 19(2)–(3) "Foreign code and unsafe boundaries", ForeignAbort
//   - rules/platform/ffi.md — §42 "Foreign effects"
func (a *Analyzer) recordForeignAbortEffect(callee Function, source lexer.Token) {
	if !callee.Extern || callee.TrustedNoPanic || a.summaryPass || !a.callGraphPathReachable {
		return
	}
	a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicForeign, Source: source, PanicReasonIDs: []diagnostics.PanicReasonID{diagnostics.PanicReasonForeignAbort}})
}

// recordForeignAllocationEffect treats a call to an extern function as having
// unknown allocation behavior unless its declaration carries a trusted
// @noAlloc foreign contract.
//
// Rules:
//   - rules/foundations/attributes.md — "Sec code versus foreign declarations", "@noAlloc verification"
//   - rules/memory/allocation.md — § 19 "FFI and foreign allocation", § 24(6)
func (a *Analyzer) recordForeignAllocationEffect(callee Function, source lexer.Token) {
	if !callee.Extern || callee.TrustedNoAlloc || a.summaryPass || !a.callGraphPathReachable {
		return
	}
	a.callGraph.addArenaEffect(a.currentCallable, ArenaEffectSite{Kind: ArenaEffectForeign, Source: source, UnknownAllocation: true})
}
