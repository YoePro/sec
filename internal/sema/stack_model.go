package sema

import (
	"math/big"

	"sec/internal/sema/stackbound"
)

// The stack-resource proof values live in the stackbound package. These names
// keep Sema's established API for call-graph composition, summaries, evidence,
// budget diagnostics and their CLI/LSP consumers.
//
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification",
// "Recursion-depth classification", "Stack budgets", "Budget validation".
type (
	StackBoundKind        = stackbound.Kind
	StackBound            = stackbound.Bound
	RecursionDepthKind    = stackbound.DepthKind
	RecursionDepthBound   = stackbound.Depth
	StackMeasurementLevel = stackbound.MeasurementLevel
	StackBudget           = stackbound.Budget
	StackBudgetResult     = stackbound.BudgetResult
)

const (
	StackBoundExact      = stackbound.Exact
	StackBoundUpperBound = stackbound.UpperBound
	StackBoundUnknown    = stackbound.Unknown
	StackBoundUnbounded  = stackbound.Unbounded

	RecursionDepthExact      = stackbound.DepthExact
	RecursionDepthUpperBound = stackbound.DepthUpperBound
	RecursionDepthUnknown    = stackbound.DepthUnknown
	RecursionDepthUnbounded  = stackbound.DepthUnbounded

	StackMeasurementSemantic = stackbound.Semantic
	StackMeasurementMachine  = stackbound.Machine

	StackBudgetSatisfied     = stackbound.Satisfied
	StackBudgetExactExcess   = stackbound.ExactExcess
	StackBudgetUpperUnproven = stackbound.UpperUnproven
	StackBudgetUnknown       = stackbound.BudgetUnknown
	StackBudgetUnbounded     = stackbound.BudgetUnbounded
)

// NewExactStackBound records a producer's proven exact nonnegative byte count.
func NewExactStackBound(bytes *big.Int) (StackBound, error) { return stackbound.NewExact(bytes) }

// NewUpperStackBound records a producer's proven finite upper byte bound.
func NewUpperStackBound(bytes *big.Int) (StackBound, error) { return stackbound.NewUpper(bytes) }

// UnknownStackBound records that no finite upper bound has been proved.
func UnknownStackBound() StackBound { return stackbound.UnknownBound() }

// UnboundedStackBound records a producer's proof of arbitrarily large demand.
func UnboundedStackBound() StackBound { return stackbound.UnboundedBound() }

// NewExactRecursionDepth records a producer's exact nonnegative depth count.
func NewExactRecursionDepth(count *big.Int) (RecursionDepthBound, error) {
	return stackbound.NewExactDepth(count)
}

// NewUpperRecursionDepth records a proven finite upper depth bound.
func NewUpperRecursionDepth(count *big.Int) (RecursionDepthBound, error) {
	return stackbound.NewUpperDepth(count)
}

// UnknownRecursionDepth records absence of a proven finite maximum depth.
func UnknownRecursionDepth() RecursionDepthBound { return stackbound.UnknownDepth() }

// UnboundedRecursionDepth records a producer's proof of unbounded depth.
func UnboundedRecursionDepth() RecursionDepthBound { return stackbound.UnboundedDepth() }

// NewStackBudget validates and detaches a finite available-stack contract.
func NewStackBudget(domain string, level StackMeasurementLevel, maximum *big.Int) (StackBudget, error) {
	return stackbound.NewBudget(domain, level, maximum)
}
