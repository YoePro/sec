package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// DeferCaptureKind separates future value access from the reference holder
// whose authority must remain available until the deferred access executes.
// Rules: rules/control-flow/defer.md — §§9–11;
// rules/memory/borrowing.md — §21(1), §21(4)–(5).
type DeferCaptureKind string

const (
	DeferCaptureValueUse        DeferCaptureKind = "value-use"
	DeferCaptureReferenceHolder DeferCaptureKind = "reference-holder"
)

// ResolvedDeferCapture retains the canonical storage path and binding selected
// during body analysis. Binding may be zero for static/external storage; Place
// still retains its canonical root and storage provenance. The capture is a
// delayed use, not a copied value or an evaluated index/range snapshot.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§42–44;
// rules/memory/lifetime_analysis.md — §17.1(1–2), §17.2(1).
type ResolvedDeferCapture struct {
	Binding       BindingID
	Place         Place
	Kind          DeferCaptureKind
	RequiredUntil CleanupActionID
	Use           lexer.Token
}

// ResolvedDeferPlan describes one validated defer site. Captured bindings and
// Places must remain valid until this site's registration executes at the
// enclosing Invocation boundary. Reads observe the then-current value. A loop
// may register this same site repeatedly with distinct dynamic binding instances;
// this plan neither counts nor collapses those registrations.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§42–44,74,79;
// rules/control-flow/defer.md — §§3,9–13.
type ResolvedDeferPlan struct {
	Identity   ResolvedDeferIdentity
	Invocation CallableID
	Body       CallableID
	Captures   []ResolvedDeferCapture
}

// recordResolvedDeferPlan publishes the captures already resolved by the defer
// body checker, without inferring types, copying owned values or resolving names
// again. The borrow checker retains these same dependencies until invocation exit.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§42–43,79;
// rules/control-flow/defer.md — §§9–11,29.
func (a *Analyzer) recordResolvedDeferPlan(stmt *ast.DeferStatement, identity ResolvedDeferIdentity, invocation, body CallableID) {
	keys := make([]string, 0, len(a.deferCaptures))
	for key := range a.deferCaptures {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	plan := ResolvedDeferPlan{Identity: identity, Invocation: invocation, Body: body}
	for _, key := range keys {
		capture := a.deferCaptures[key]
		kind := DeferCaptureValueUse
		if capture.Kind == deferredReferenceHolderUse {
			kind = DeferCaptureReferenceHolder
		}
		plan.Captures = append(plan.Captures, ResolvedDeferCapture{
			Binding: a.bindingIDs[sourceTokenLocation(capture.Place.RootToken)],
			Place:   cloneSlicePlanPlace(capture.Place), Kind: kind,
			RequiredUntil: identity.Action, Use: capture.Token,
		})
	}
	if a.resolvedDeferPlans == nil {
		a.resolvedDeferPlans = map[*ast.DeferStatement]ResolvedDeferPlan{}
	}
	a.resolvedDeferPlans[stmt] = plan
}

// cloneResolvedDeferPlan detaches nested Place/type/range facts while retaining
// the semantic identities needed by cleanup consumers.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§42–43,79.
func cloneResolvedDeferPlan(plan ResolvedDeferPlan) ResolvedDeferPlan {
	if plan.Captures != nil {
		captures := make([]ResolvedDeferCapture, len(plan.Captures))
		copy(captures, plan.Captures)
		for index := range captures {
			captures[index].Place = cloneSlicePlanPlace(captures[index].Place)
		}
		plan.Captures = captures
	}
	return plan
}

// ResolvedDeferPlanOf returns detached validated captures and their required
// invocation lifetime without inference, identity allocation or flow mutation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§42–43,79.
func (a *Analyzer) ResolvedDeferPlanOf(stmt *ast.DeferStatement) (ResolvedDeferPlan, bool) {
	if a == nil || stmt == nil {
		return ResolvedDeferPlan{}, false
	}
	plan, ok := a.resolvedDeferPlans[stmt]
	if !ok {
		return ResolvedDeferPlan{}, false
	}
	return cloneResolvedDeferPlan(plan), true
}
