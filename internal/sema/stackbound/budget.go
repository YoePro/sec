package stackbound

import (
	"errors"
	"math/big"
)

// MeasurementLevel identifies the authoritative resource measurement.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "Stack analysis levels".
type MeasurementLevel string

const (
	Semantic MeasurementLevel = "Semantic"
	Machine  MeasurementLevel = "Machine"
)

// Budget is an immutable finite available-stack contract. Domain is an
// opaque physical-stack identity supplied by the runtime/platform producer,
// not a callable or logical execution root. No configuration syntax is implied.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "Physical stack domains".
type Budget struct {
	domain  string
	level   MeasurementLevel
	maximum string
}

// NewBudget validates and detaches an available-byte count, including zero.
// Only explicit domains and supported measurement levels form usable contracts.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "Budget validation".
func NewBudget(domain string, level MeasurementLevel, maximum *big.Int) (Budget, error) {
	if domain == "" || (level != Semantic && level != Machine) || maximum == nil || maximum.Sign() < 0 {
		return Budget{}, errors.New("stack budget requires a physical domain, semantic or machine level and nonnegative maximum bytes")
	}
	return Budget{domain: domain, level: level, maximum: maximum.String()}, nil
}

// Domain returns the contract's physical-stack identity without inferring root nesting.
// Rules: rules/analysis/stack_analysis.md — "Physical stack domains" and "Stack budgets".
func (budget Budget) Domain() string { return budget.domain }

// MeasurementLevel identifies which verified requirement may satisfy the contract.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "Machine-level revalidation".
func (budget Budget) MeasurementLevel() MeasurementLevel { return budget.level }

// MaximumBytes returns a detached finite availability count, or absence for an
// uninitialized budget. Absence must never become an active zero-byte contract.
// Rules: rules/analysis/stack_analysis.md — "Stack budgets" and "No-budget behavior".
func (budget Budget) MaximumBytes() (*big.Int, bool) {
	maximum, ok := new(big.Int).SetString(budget.maximum, 10)
	return maximum, ok
}

// BudgetResult preserves the difference between a proven excess and a
// failed sufficiency proof. An upper bound above availability proves no excess.
// These facts are not diagnostics or an implicit build policy.
// Rules: rules/analysis/stack_analysis.md — "Budget validation", "UpperBound", and "No-budget behavior".
type BudgetResult string

const (
	Satisfied       BudgetResult = "Satisfied"
	ExactExcess     BudgetResult = "ExactExcess"
	UpperUnproven   BudgetResult = "UpperBoundUnproven"
	BudgetUnknown   BudgetResult = "Unknown"
	BudgetUnbounded BudgetResult = "Unbounded"
)

// Compare requires a producer-verified requirement for this physical domain and
// authoritative measurement level. Producers remain responsible for root
// composition, plan validity and final backend evidence; semantic estimates
// cannot satisfy machine contracts. Invalid or mismatched inputs return errors,
// never a satisfied result. Calling this API does not activate a build policy.
// Rules: rules/analysis/stack_analysis.md — "Budget validation", "Physical stack domains",
// "CompilationPlan dependence", "Machine-level revalidation", and "No-budget behavior".
func (budget Budget) Compare(domain string, level MeasurementLevel, required Bound) (BudgetResult, error) {
	maximum, valid := budget.MaximumBytes()
	if !valid || budget.domain == "" || (budget.level != Semantic && budget.level != Machine) {
		return "", errors.New("stack budget is not initialized")
	}
	if domain != budget.domain || level != budget.level {
		return "", errors.New("stack requirement does not match budget domain and measurement level")
	}
	switch required.Kind() {
	case Unknown:
		return BudgetUnknown, nil
	case Unbounded:
		return BudgetUnbounded, nil
	}
	bytes, finite := required.Bytes()
	if !finite {
		return "", errors.New("stack requirement has no valid finite byte count")
	}
	if bytes.Cmp(maximum) <= 0 {
		return Satisfied, nil
	}
	if required.Kind() == Exact {
		return ExactExcess, nil
	}
	return UpperUnproven, nil
}
