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
	Site            TransferSiteKind
	Action          TransferAction
	Source          Place
	Partial         bool
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
	if a.resolvedTransferPlans == nil {
		a.resolvedTransferPlans = map[ast.Expression]ResolvedTransferPlan{}
	}
	token := expressionToken(value)
	after := TransferInitialized
	if action == TransferMove {
		after = TransferMoved
	}
	a.resolvedTransferPlans[value] = ResolvedTransferPlan{
		Site: site, Action: action, Source: cloneSlicePlanPlace(place),
		Partial: action == TransferMove && len(place.Projections) > 0,
		File:    token.File, Line: token.Line, Column: token.Column,
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
