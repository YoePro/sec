package sema

import (
	"errors"
	"math/big"
)

// StackMeasurementLevel identifies the authoritative resource measurement.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "Stack analysis levels".
type StackMeasurementLevel string

const (
	StackMeasurementSemantic StackMeasurementLevel = "Semantic"
	StackMeasurementMachine  StackMeasurementLevel = "Machine"
)

// StackBudget is an immutable finite available-stack contract. Domain is an
// opaque physical-stack identity supplied by the runtime/platform producer,
// not a callable or logical execution root. No configuration syntax is implied.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "Physical stack domains".
type StackBudget struct {
	domain  string
	level   StackMeasurementLevel
	maximum string
}

// NewStackBudget validates and detaches an available-byte count, including zero.
// Only explicit domains and supported measurement levels form usable contracts.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "Budget validation".
func NewStackBudget(domain string, level StackMeasurementLevel, maximum *big.Int) (StackBudget, error) {
	if domain == "" || (level != StackMeasurementSemantic && level != StackMeasurementMachine) || maximum == nil || maximum.Sign() < 0 {
		return StackBudget{}, errors.New("stack budget requires a physical domain, semantic or machine level and nonnegative maximum bytes")
	}
	return StackBudget{domain: domain, level: level, maximum: maximum.String()}, nil
}

// Domain returns the contract's physical-stack identity without inferring root nesting.
// Rules: rules/analysis/stack_analysis.md — "Physical stack domains" and "Stack budgets".
func (budget StackBudget) Domain() string { return budget.domain }

// MeasurementLevel identifies which verified requirement may satisfy the contract.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "Machine-level revalidation".
func (budget StackBudget) MeasurementLevel() StackMeasurementLevel { return budget.level }

// MaximumBytes returns a detached finite availability count, or absence for an
// uninitialized budget. Absence must never become an active zero-byte contract.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "No-budget behavior".
func (budget StackBudget) MaximumBytes() (*big.Int, bool) {
	maximum, ok := new(big.Int).SetString(budget.maximum, 10)
	return maximum, ok
}

// StackBudgetResult preserves the difference between a proven excess and a
// failed sufficiency proof. An upper bound above availability proves no excess.
// These facts are not diagnostics or an implicit build policy.
// Rules: rules/analysis/stack_analysis.md — "Budget validation", "UpperBound", and "No-budget behavior".
type StackBudgetResult string

const (
	StackBudgetSatisfied     StackBudgetResult = "Satisfied"
	StackBudgetExactExcess   StackBudgetResult = "ExactExcess"
	StackBudgetUpperUnproven StackBudgetResult = "UpperBoundUnproven"
	StackBudgetUnknown       StackBudgetResult = "Unknown"
	StackBudgetUnbounded     StackBudgetResult = "Unbounded"
)

// Compare requires a producer-verified requirement for this physical domain and
// authoritative measurement level. Producers remain responsible for root
// composition, plan validity and final backend evidence; semantic estimates
// cannot satisfy machine contracts. Invalid or mismatched inputs return errors,
// never a satisfied result. Calling this API does not activate a build policy.
// Rules: rules/analysis/stack_analysis.md — "Budget validation", "Physical stack domains",
// "CompilationPlan dependence", "Machine-level revalidation", and "No-budget behavior".
func (budget StackBudget) Compare(domain string, level StackMeasurementLevel, required StackBound) (StackBudgetResult, error) {
	maximum, valid := budget.MaximumBytes()
	if !valid || budget.domain == "" || (budget.level != StackMeasurementSemantic && budget.level != StackMeasurementMachine) {
		return "", errors.New("stack budget is not initialized")
	}
	if domain != budget.domain || level != budget.level {
		return "", errors.New("stack requirement does not match budget domain and measurement level")
	}
	switch required.Kind() {
	case StackBoundUnknown:
		return StackBudgetUnknown, nil
	case StackBoundUnbounded:
		return StackBudgetUnbounded, nil
	}
	bytes, finite := required.Bytes()
	if !finite {
		return "", errors.New("stack requirement has no valid finite byte count")
	}
	if bytes.Cmp(maximum) <= 0 {
		return StackBudgetSatisfied, nil
	}
	if required.Kind() == StackBoundExact {
		return StackBudgetExactExcess, nil
	}
	return StackBudgetUpperUnproven, nil
}
