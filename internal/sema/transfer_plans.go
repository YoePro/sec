package sema

import (
	"sort"

	"sec/internal/ast"
)

// TransferSiteKind is the syntactic role of an ownership transfer.
type TransferSiteKind string

const (
	TransferSiteInitialization TransferSiteKind = "initialization"
	TransferSiteAssignment     TransferSiteKind = "assignment"
	TransferSiteArgument       TransferSiteKind = "argument"
	TransferSiteReturn         TransferSiteKind = "return"
	TransferSiteConstruction   TransferSiteKind = "construction"
	TransferSiteCapture        TransferSiteKind = "capture"
	// TransferSiteBranch is the result value of a match arm in a value
	// context, which the arm's selected edge hands to the match result.
	TransferSiteBranch TransferSiteKind = "branch"
	// TransferSiteMatchBinding is a by-value payload or field binding of a
	// match arm, `if ... is` binding or try handler, keyed by the bound
	// identifier of the pattern.
	TransferSiteMatchBinding TransferSiteKind = "match-binding"
)

// ResolvedTransferPlan is the immutable ownership plan of one validated
// transfer from a reusable source Place (P17-12, P17-13): the site, the
// resolved action (trivial copy, semantic copy or move), the source Place and
// whether the move is partial (a projection of a tracked local struct), the
// owning destination continuation and the source's planned state transition. A
// copy keeps the source; a move removes the source's responsibility and the
// destination becomes the continuation.
// Before/After describe the accepted transfer at commit, not an assertion that
// argument preparation has already committed the complete enclosing call.
//
// Rules:
//   - rules/memory/ownership.md — §§ 10–14
//   - rules/memory/copy_move.md — §§ 3, 5–10
//   - rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 5–8, 15–18
type ResolvedTransferPlan struct {
	Site    TransferSiteKind
	Action  TransferAction
	Source  Place
	Partial bool
	// SourceFresh marks a source value without a reusable Place, such as the
	// error a try handler binds or the payload of a fresh match subject
	// that has no materialized storage; Source then carries only its type.
	SourceFresh     bool
	File            string
	Line            int
	Column          int
	Destination     ResolvedTransferDestination
	OwnershipBefore TransferOwnershipState
	OwnershipAfter  TransferOwnershipState
}

// recordTransferPlan publishes the plan of a transfer that
// validateNamedOwnershipSource accepted, detaching typed Place facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§17–18;
// rules/compiler/compiler_analysis.md — §55 "Analysis snapshot consistency".
func (a *Analyzer) recordTransferPlan(value ast.Expression, place Place, site TransferSiteKind, action TransferAction) {
	if a == nil || value == nil {
		return
	}
	if _, _, _, unavailable := a.unavailablePlace(place); unavailable {
		return
	}
	a.publishTransferPlan(value, place, false, site, action)
}

// publishTransferPlan stores an accepted transfer. Pattern bindings publish
// through it directly: a moved payload Place is already unavailable when the
// binding is defined.
func (a *Analyzer) publishTransferPlan(value ast.Expression, place Place, fresh bool, site TransferSiteKind, action TransferAction) {
	if a.resolvedTransferPlans == nil {
		a.resolvedTransferPlans = map[ast.Expression]ResolvedTransferPlan{}
	}
	token := expressionToken(value)
	after := TransferInitialized
	if action == TransferMove {
		after = TransferMoved
		if !fresh {
			a.noteLambdaEnvironmentConsume(place, token)
		}
	}
	a.resolvedTransferPlans[value] = ResolvedTransferPlan{
		Site: site, Action: action, Source: cloneSlicePlanPlace(place),
		Partial: action == TransferMove && len(place.Projections) > 0, SourceFresh: fresh,
		File: token.File, Line: token.Line, Column: token.Column,
		Destination:     a.transferDestination(value, site),
		OwnershipBefore: TransferInitialized, OwnershipAfter: after,
	}
}

// ResolvedTransferPlanOf returns a detached copy of the transfer plan of a
// source expression; mutating it never changes the analyzer's fact (P17-14).
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §18.
func (a *Analyzer) ResolvedTransferPlanOf(value ast.Expression) (ResolvedTransferPlan, bool) {
	if a == nil || value == nil {
		return ResolvedTransferPlan{}, false
	}
	plan, ok := a.resolvedTransferPlans[value]
	if !ok {
		return ResolvedTransferPlan{}, false
	}
	return cloneTransferPlan(plan), true
}

// ResolvedTransferPlans lists every transfer plan in source order, detached.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §18.
func (a *Analyzer) ResolvedTransferPlans() []ResolvedTransferPlan {
	if a == nil {
		return nil
	}
	plans := make([]ResolvedTransferPlan, 0, len(a.resolvedTransferPlans))
	for _, plan := range a.resolvedTransferPlans {
		plans = append(plans, cloneTransferPlan(plan))
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].File != plans[j].File {
			return plans[i].File < plans[j].File
		}
		if plans[i].Line != plans[j].Line {
			return plans[i].Line < plans[j].Line
		}
		if plans[i].Column != plans[j].Column {
			return plans[i].Column < plans[j].Column
		}
		return plans[i].Destination.ID < plans[j].Destination.ID
	})
	return plans
}

// transferActionOf is the transfer a by-value binding or accepted copy of a
// value of typ performs, mirroring the copy classification Sema already
// validated. A conditional or unknown classification has no plan.
func transferActionOf(typ Type) (TransferAction, bool) {
	switch CopyClassificationOf(typ) {
	case CopyTrivial:
		return TransferCopyTrivial, true
	case CopySemantic:
		return TransferCopySemantic, true
	case CopyMoveOnly, CopyNonCopyable:
		return TransferMove, true
	}
	return "", false
}

// recordBindingTransfer publishes the transfer of a by-value pattern binding
// from its source Place (or a fresh source value) into the binding's own
// local Place (P17-13). A borrowing binding is not a transfer; it publishes a
// reference plan instead.
func (a *Analyzer) recordBindingTransfer(node *ast.Identifier, name string, source Place, typ Type) {
	if a == nil || node == nil || name == "" || name == "_" || typ.Kind == InvalidType || typ.Kind == ReferenceType {
		return
	}
	action, ok := transferActionOf(typ)
	if !ok {
		return
	}
	fresh := source.Root == ""
	if fresh {
		source = Place{Type: typ}
	}
	a.publishTransferPlan(node, source, fresh, TransferSiteMatchBinding, action)
	if destination, ok := a.rootPlace(name); ok {
		a.bindTransferDestinationPlace(node, destination)
	}
}

// recordAcceptedCopy publishes an accepted copy of a reusable source Place at
// a return, construction or branch site (P17-13). Moves publish through
// validateNamedOwnershipSource; a source needing a transfer has no copy.
func (a *Analyzer) recordAcceptedCopy(expr ast.Expression, place Place, site TransferSiteKind) {
	if _, recorded := a.resolvedTransferPlans[expr]; recorded {
		return
	}
	action, ok := transferActionOf(place.Type)
	if !ok || action == TransferMove {
		return
	}
	a.recordTransferPlan(expr, place, site, action)
}

// relabelTransferSite moves an accepted plan to the site that owns its
// destination, for an arm result whose explicit move was validated as a
// construction operand.
func (a *Analyzer) relabelTransferSite(expr ast.Expression, site TransferSiteKind) {
	plan, ok := a.resolvedTransferPlans[expr]
	if !ok || plan.Site == site {
		return
	}
	plan.Site = site
	plan.Destination = a.transferDestination(expr, site)
	a.resolvedTransferPlans[expr] = plan
}

// OwnershipUseKind classifies how one source use treats ownership (P17-13;
// package17 § 16): an owning transfer, a borrow that creates a reference, a
// use through an existing reference, or a non-consuming observation of a
// Place that neither transfers nor borrows it.
type OwnershipUseKind string

const (
	OwnershipUseTransfer      OwnershipUseKind = "transfer"
	OwnershipUseBorrowShared  OwnershipUseKind = "borrow-shared"
	OwnershipUseBorrowMutable OwnershipUseKind = "borrow-mutable"
	OwnershipUseReference     OwnershipUseKind = "reference"
	OwnershipUseObservation   OwnershipUseKind = "observation"
)

// ResolvedOwnershipUse is the classified use of one expression. Transfer is
// set only for a transfer; Place is the used Place.
type ResolvedOwnershipUse struct {
	Kind     OwnershipUseKind
	Transfer *ResolvedTransferPlan
	Place    Place
}

// ResolvedOwnershipUseOf classifies the use of an expression from facts
// Sema already published: its transfer plan, its reference creation or
// reference-mediated use, or else its analyzed Place as an observation. It
// resolves nothing again; an expression without such facts has no use.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 16–18;
// rules/memory/copy_move.md — § 3; rules/memory/borrowing.md — § 15.
func (a *Analyzer) ResolvedOwnershipUseOf(expr ast.Expression) (ResolvedOwnershipUse, bool) {
	if a == nil || expr == nil {
		return ResolvedOwnershipUse{}, false
	}
	if plan, ok := a.ResolvedTransferPlanOf(expr); ok {
		return ResolvedOwnershipUse{Kind: OwnershipUseTransfer, Transfer: &plan, Place: cloneSlicePlanPlace(plan.Source)}, true
	}
	if plan, ok := a.ResolvedReferencePlanOf(expr); ok {
		kind := OwnershipUseBorrowShared
		if plan.Facts.MutableAuthority {
			kind = OwnershipUseBorrowMutable
		}
		return ResolvedOwnershipUse{Kind: kind, Place: plan.Place}, true
	}
	if use, ok := a.ResolvedReferenceUseOf(expr); ok {
		return ResolvedOwnershipUse{Kind: OwnershipUseReference, Place: use.Place}, true
	}
	if place, ok := a.ResolvedPlaceOf(expr); ok {
		return ResolvedOwnershipUse{Kind: OwnershipUseObservation, Place: place}, true
	}
	return ResolvedOwnershipUse{}, false
}

// constructionOperandAction reads the transfer Sema resolved for one owning
// construction operand: an explicit `<-` or an implicit move of a reusable
// Place moves it, a published copy copies it, and any other operand is a fresh
// value the construction takes directly (copied when trivially copyable). It
// reads published plans and type facts only (P17-35).
func (a *Analyzer) constructionOperandAction(expr ast.Expression, typ Type) ResolvedStructFieldAction {
	if move, explicit := explicitMoveArgument(expr); explicit {
		expr = move.Right
	}
	if plan, ok := a.resolvedTransferPlans[expr]; ok {
		switch plan.Action {
		case TransferCopyTrivial:
			return StructFieldCopyTrivial
		case TransferCopySemantic:
			return StructFieldCopySemanticInfallible
		default:
			return StructFieldMove
		}
	}
	if CopyClassificationOf(typ) == CopyTrivial {
		return StructFieldCopyTrivial
	}
	return StructFieldConstructDirect
}

// ResolvedTryHandlerBindingTransferOf reads the transfer of a try handler's
// by-value binding of the error or of an error variant's payload (P17-13,
// P17-35), detached from analysis state.
func (a *Analyzer) ResolvedTryHandlerBindingTransferOf(handler *ast.TryHandler) (ResolvedTransferPlan, bool) {
	if a == nil || handler == nil {
		return ResolvedTransferPlan{}, false
	}
	node, ok := a.tryHandlerBindingNodes[handler]
	if !ok {
		return ResolvedTransferPlan{}, false
	}
	return a.ResolvedTransferPlanOf(node)
}
