package sema

import "fmt"

// CopyAdvice describes the evidence for an optional copy-cost recommendation.
// Its size comes from the active plan's canonical layout and is cost evidence
// only, never an ABI guarantee or ownership rule.
// The same wording serves compiler diagnostics and detailed LSP hover.
// Rules: rules/memory/copy_move.md §22(1–6);
// rules/analysis/parameter_usage_analysis.md §§63–65,92.
func (recommendation ParameterRecommendation) CopyAdvice() string {
	if recommendation.Status != "recommended" || !recommendation.SizeKnown {
		return ""
	}
	size := fmt.Sprintf("Estimated copy size: %d bytes (cost estimate, not a layout guarantee).", recommendation.EstimatedSizeBytes)
	if recommendation.SizeSource == ParameterSizeFromLayout {
		size = fmt.Sprintf("Copy size on the active target: %d bytes (from the target layout; cost evidence, not an ABI guarantee).", recommendation.EstimatedSizeBytes)
	}
	return fmt.Sprintf("%s The function only needs call-local read access without retaining the value or requiring ownership or address identity. The by-value copy is legal; changing to %s is an optional API design choice, not an ownership repair.", size, recommendation.Candidate)
}
