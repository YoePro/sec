package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// recordSliceRangeEffect publishes only a resolved unproven spatial check as
// BoundsFailure. Reborrowing may independently require dynamic reference
// validation; try removes only this spatial effect. Neither check adds an
// allocation/blocking effect, and operand/call effects remain in their owners.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§45–46,118–120;
// rules/errors/runtime_checks.md — "What try converts";
// rules/errors/panic.md — §§21,25.
func (a *Analyzer) recordSliceRangeEffect(expr *ast.RefExpression, plan ResolvedSliceCreationPlan) {
	if a.summaryPass || expr == nil {
		return
	}
	a.callGraph.removeEffect(a.currentCallable, EffectMayPanicBounds, expr.Token)
	if !a.callGraphPathReachable || plan.CheckKind != SliceRangeRuntimeCheck {
		return
	}
	a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicBounds, Source: expr.Token, PanicReasonIDs: []diagnostics.PanicReasonID{diagnostics.PanicReasonBoundsFailure}})
}

// recordSliceLengthUse records the bounded reference observation after the
// compiler-known Len/len operation has resolved. It uses the existing P15
// reference facts and effect producer; it neither reads elements nor allocates.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§45,47–49,118–119.
func (a *Analyzer) recordSliceLengthUse(expr, source ast.Expression) {
	if a.summaryPass {
		return
	}
	if place, ok := a.resolvePlace(source); ok {
		a.recordReferenceUse(expr, ReferenceUseRead, place, false, expressionToken(expr))
	}
}
