package sema

import (
	"fmt"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// DiagnoseStackBudget renders one mandatory failure for an explicitly active
// finite budget, or no diagnostic for absent/satisfied contracts. Requirement
// producers must supply verified physical-domain facts at the authoritative
// measurement level and current plan, including final frames for machine budgets.
// Scope mismatches return API errors rather than fabricating proof failures.
// Source identifies the root or contract being validated; evidence may identify
// an uncertainty boundary as a related location. This API does not read project
// configuration or activate stack validation in the default compiler pipeline.
// Callers must exclude snapshots already invalidated by upstream language
// errors, so stack findings do not duplicate lifetime/layout/call-graph errors.
// Rules: rules/analysis/stack_analysis.md — "Budget validation", "No-budget behavior",
// "Machine-level revalidation", "Diagnostics", and "Diagnostic ownership and severity";
// rules/tooling/diagnostics.md — §5 stable IDs, §7 occurrences, §9 locations.
func DiagnoseStackBudget(budget *StackBudget, domain string, level StackMeasurementLevel, required StackBound, source lexer.Token, evidence StackEvidence) (*Error, error) {
	if budget == nil {
		return nil, nil
	}
	result, err := budget.Compare(domain, level, required)
	if err != nil {
		return nil, err
	}
	if result == StackBudgetSatisfied {
		return nil, nil
	}
	available, _ := budget.MaximumBytes()
	endLine, endColumn := source.EndPosition()
	diagnostic := &Error{Severity: diagnostics.SeverityError,
		File: source.File, Line: source.Line, Column: source.Column, EndLine: endLine, EndColumn: endColumn}
	switch result {
	case StackBudgetExactExcess:
		diagnostic.ID = diagnostics.StackBudgetExceeded
		bytes, _ := required.Bytes()
		diagnostic.Message = fmt.Sprintf("%s stack budget exceeded for domain %q: required %s B, available %s B", level, domain, bytes, available)
		diagnostic.Help = "Reduce simultaneously live stack demand or revise the explicit stack budget under the applicable target/project contract."
	case StackBudgetUpperUnproven:
		diagnostic.ID = diagnostics.StackBudgetProofUnavailable
		bytes, _ := required.Bytes()
		diagnostic.Message = fmt.Sprintf("cannot prove %s stack fits budget for domain %q: verified upper bound %s B, available %s B", level, domain, bytes, available)
		diagnostic.Help = "An upper bound above the budget does not prove actual excess or inevitable overflow. Establish a tighter verified bound at the budget's authoritative measurement level or revise the explicit contract."
	case StackBudgetUnknown:
		diagnostic.ID = diagnostics.StackBudgetProofUnavailable
		diagnostic.Message = fmt.Sprintf("finite %s stack bound could not be proven for domain %q: required budget <= %s B", level, domain, available)
		diagnostic.Help = "The active budget requires a verified finite bound. Unknown does not prove stack overflow; supply the missing frame, depth or callable/runtime contract evidence."
		causes := mergeStackEvidence(StackEvidence{}, evidence).UnknownCauses
		if len(causes) > 0 {
			cause := causes[0]
			diagnostic.Help += " Boundary: " + cause.Detail
			if cause.Source.Line > 0 && cause.Source.Column > 0 {
				diagnostic.PreviousFile, diagnostic.PreviousLine, diagnostic.PreviousColumn = cause.Source.File, cause.Source.Line, cause.Source.Column
				diagnostic.RelatedLabel = "stack proof boundary"
			}
		}
	case StackBudgetUnbounded:
		diagnostic.ID = diagnostics.StackBudgetUnboundedDemand
		diagnostic.Message = fmt.Sprintf("proven unbounded %s stack demand cannot satisfy finite budget for domain %q: available %s B", level, domain, available)
		diagnostic.Help = "Establish a finite bound on the reachable execution structure under the active execution model. Increasing a finite byte budget cannot satisfy proven unbounded demand."
	default:
		return nil, fmt.Errorf("unsupported stack budget comparison result %q", result)
	}
	return diagnostic, nil
}
