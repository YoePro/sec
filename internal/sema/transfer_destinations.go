package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// TransferOwnershipState is the state of the exact source value/Place on the
// two sides of a resolved transfer, rather than the availability of its entire
// enclosing aggregate. Partial-owner masks remain a separate fact producer.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§10–11,17.
type TransferOwnershipState string

const (
	TransferInitialized TransferOwnershipState = "initialized"
	TransferMoved       TransferOwnershipState = "moved"
)

// TransferDestinationID identifies one semantic owning continuation at a use
// site within an analysis. It is neither a physical address nor a dynamic loop
// instance. Canonical Places and selected callees refine that destination.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§9,17,67–69.
type TransferDestinationID uint32

// ResolvedTransferDestination distinguishes local storage, a selected argument
// slot, a return slot, an owning construction result and a closure capture slot.
// Site and ID identify the continuation even when it has no addressable Place.
// Callable identifies the selected callee for arguments and current callable
// for returns; Context is always the callable containing the source use.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§17–18,60–61,67–69;
// rules/memory/copy_move.md — §§8–11.
type ResolvedTransferDestination struct {
	ID            TransferDestinationID
	Site          TransferSiteKind
	Context       CallableID
	Place         *Place
	Callable      CallableID
	ArgumentIndex int
	Parameter     lexer.Token
}

type transferDestinationKey struct {
	Use     ast.Expression
	Site    TransferSiteKind
	Context CallableID
}

// transferDestination interns an owning use continuation during production.
// Querying an already published plan never allocates another identity.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§9,17–18.
func (a *Analyzer) transferDestination(value ast.Expression, site TransferSiteKind) ResolvedTransferDestination {
	// Distinct synthetic use nodes may share a source token. Node identity
	// separates those uses; allocation order, never an address, defines the ID.
	key := transferDestinationKey{value, site, a.currentCallable}
	if a.transferDestinations == nil {
		a.transferDestinations = map[transferDestinationKey]TransferDestinationID{}
	}
	id, ok := a.transferDestinations[key]
	if !ok {
		id = TransferDestinationID(len(a.transferDestinations) + 1)
		a.transferDestinations[key] = id
	}
	destination := ResolvedTransferDestination{ID: id, Site: site, Context: a.currentCallable}
	if site == TransferSiteReturn {
		destination.Callable = a.currentCallable
	}
	return destination
}

// bindTransferDestinationPlace retains the canonical destination selected by
// successful declaration/assignment validation. It never changes the action.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §17;
// rules/memory/copy_move.md — §§5,10,12.
func (a *Analyzer) bindTransferDestinationPlace(value ast.Expression, place Place) {
	plan, ok := a.resolvedTransferPlans[value]
	if !ok {
		return
	}
	copy := cloneSlicePlanPlace(place)
	plan.Destination.Place = &copy
	a.resolvedTransferPlans[value] = plan
}

// bindTransferArgumentDestination preserves the already selected overload and
// source argument position without resolving a signature again in lowering.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§17,67;
// rules/memory/copy_move.md — §8.
func (a *Analyzer) bindTransferArgumentDestination(value ast.Expression, function Function, index int, parameter FunctionParameter) {
	plan, ok := a.resolvedTransferPlans[value]
	if !ok {
		return
	}
	plan.Destination.Callable = callableID(function)
	plan.Destination.ArgumentIndex = index
	plan.Destination.Parameter = parameter.Token
	a.resolvedTransferPlans[value] = plan
}

// cloneTransferPlan detaches both source and destination typed Places at the
// immutable query boundary, retaining exact paths and semantic identities.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §18.
func cloneTransferPlan(plan ResolvedTransferPlan) ResolvedTransferPlan {
	plan.Source = cloneSlicePlanPlace(plan.Source)
	if plan.Destination.Place != nil {
		place := cloneSlicePlanPlace(*plan.Destination.Place)
		plan.Destination.Place = &place
	}
	return plan
}
