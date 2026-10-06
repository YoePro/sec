package sema

import (
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// ParameterRecommendation describes the policy result for a shared-reference
// candidate independently of the semantic demand. Blocked candidates carry no
// confidence claim; Strong recommendations remain API design choices.
// Rules: rules/analysis/parameter_usage_analysis.md — "Semantic demand and recommendation policy are separate",
// "Recommendation confidence", "Recommendation reasons", and "Candidate blockers".
type ParameterRecommendation struct {
	Callable           CallableID
	CallableName       string
	Parameter          string
	Binding            BindingID
	Source             lexer.Token
	Candidate          string
	Status             string
	Confidence         string
	Reasons            []string
	Blockers           []string
	EstimatedSizeBytes int64
	SizeKnown          bool
}

// Recommendations returns fresh, deterministically ordered policy results for
// the explicit by-value parameters covered by the current shared-borrow advice.
// Rules: rules/analysis/parameter_usage_analysis.md — "Large value to reference",
// "Recommendation confidence", "Candidate blockers", and "Blocked narrowing advisory".
func (p *ParameterUsageAnalysis) Recommendations() []ParameterRecommendation {
	if p == nil {
		return nil
	}
	var results []ParameterRecommendation
	for _, id := range p.summaryOrder {
		summary := p.summaries[id]
		if summary == nil {
			continue
		}
		for _, parameter := range summary.Parameters {
			typ := parameter.DeclaredType
			if parameter.DeclaredRef || typ.Kind == ReferenceType || typ.Kind == SliceType || typ.Kind == VoidType || typ.Kind == InvalidType {
				continue
			}
			result := sharedReferenceRecommendation(parameter)
			if summary.Precision != ParameterDemandExact {
				result.Status = "blocked"
				result.Confidence = ""
				result.Reasons = nil
				result.SizeKnown = false
				result.EstimatedSizeBytes = 0
				result.Blockers = append(result.Blockers, "callable demand precision is "+string(summary.Precision))
			}
			result.Callable = id
			result.CallableName = summary.Name
			results = append(results, result)
		}
	}
	return results
}

// sharedReferenceRecommendation applies cost policy only after capability
// preservation is proven. An estimated size is cost evidence, never a semantic
// proof or a Certain claim about programmer intent.
// Rules: rules/analysis/parameter_usage_analysis.md — "Large-value advisory",
// "ResolvedLayout as cost input", and "Recommendation confidence".
func sharedReferenceRecommendation(parameter ParameterUsageParameterSummary) ParameterRecommendation {
	result := ParameterRecommendation{
		Parameter: parameter.Name, Binding: parameter.Binding, Source: parameter.Declaration,
		Candidate: "ref " + typeDisplayName(parameter.DeclaredType),
		Blockers:  sharedReferenceCandidateBlockers(parameter),
	}
	if len(result.Blockers) != 0 {
		result.Status = "blocked"
		return result
	}
	result.EstimatedSizeBytes, result.SizeKnown = estimatedTypeSizeBytes(parameter.DeclaredType, map[string]bool{})
	if !result.SizeKnown {
		result.Status = "not-preferred"
		result.Reasons = []string{"copy cost is unknown"}
		return result
	}
	if result.EstimatedSizeBytes < largeByValueParameterThresholdBytes {
		result.Status = "not-preferred"
		result.Reasons = []string{"value is below the large-value cost threshold"}
		return result
	}
	result.Status = "recommended"
	result.Confidence = "strong"
	result.Reasons = []string{"AvoidCopyCost", "modeled demand requires only call-local read access without ownership or address identity"}
	return result
}

// sharedReferenceCandidateBlockers explains every failed critical capability
// test in a fixed order, including localized unknowns. ExactExtent remains
// compatible with ref T[N], which preserves that contract.
// Rules: rules/analysis/parameter_usage_analysis.md — "Candidate narrowing",
// "Unknown critical dimensions block narrowing", and "Candidate blockers".
func sharedReferenceCandidateBlockers(parameter ParameterUsageParameterSummary) []string {
	demand := parameter.Demand
	var blockers []string
	if parameter.DeclaredRef || parameter.DeclaredType.Kind == ReferenceType {
		blockers = append(blockers, "parameter is already a reference")
	}
	if parameter.DeclaredMut {
		blockers = append(blockers, "declared mutability is not preserved")
	}
	if parameter.Consuming {
		blockers = append(blockers, "declared consumption is not preserved")
	}
	if parameterTypeHasCustomCleanup(parameter.DeclaredType, map[string]bool{}) {
		blockers = append(blockers, "explicit free lifecycle ownership is not preserved")
	}
	if demand.Precision != ParameterDemandExact {
		blockers = append(blockers, "demand precision is "+string(demand.Precision))
	}
	if demand.Access != ParameterAccessUnused && demand.Access != ParameterAccessRead {
		blockers = append(blockers, "access demand is "+string(demand.Access))
	}
	if demand.Mutation != ParameterNoMutation {
		blockers = append(blockers, "mutation demand is "+string(demand.Mutation))
	}
	if demand.Ownership != ParameterBorrowSufficient {
		blockers = append(blockers, "ownership demand is "+string(demand.Ownership))
	}
	if demand.Lifetime != ParameterLifetimeCallOnly {
		blockers = append(blockers, "lifetime demand is "+string(demand.Lifetime))
	}
	if demand.Identity != ParameterValueOnly {
		blockers = append(blockers, "identity demand is "+string(demand.Identity))
	}
	if demand.Representation != ParameterRepresentationNone {
		blockers = append(blockers, "representation demand is "+string(demand.Representation))
	}
	for _, storage := range demand.Storage {
		if storage != ParameterStorageNone {
			blockers = append(blockers, "storage demand is "+string(storage))
		}
	}
	if parameterDemandHasShape(demand, ParameterShapeUnknown) {
		blockers = append(blockers, "shape demand is unknown")
	}
	return blockers
}

const largeByValueParameterThresholdBytes int64 = 64

// emitLargeValueParameterAdvisories emits A2001 only after the complete local
// and interprocedural demand fixed point proves that a shared borrow preserves
// every currently modeled critical capability. Size is cost evidence only.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Candidate narrowing"
//   - rules/analysis/parameter_usage_analysis.md — "Unknown critical dimensions block narrowing"
//   - rules/analysis/parameter_usage_analysis.md — "Large-value advisory"
func (a *Analyzer) emitLargeValueParameterAdvisories() {
	if a == nil || a.parameterUsageAnalysis == nil {
		return
	}
	for _, id := range a.parameterUsageAnalysis.summaryOrder {
		summary := a.parameterUsageAnalysis.summaries[id]
		if summary == nil || summary.Precision != ParameterDemandExact {
			continue
		}
		for _, parameter := range summary.Parameters {
			a.emitLargeValueParameterAdvisory(parameter)
		}
	}
}

// emitLargeValueParameterAdvisory combines a proven shared-borrow semantic
// candidate with the current size policy without altering ParameterDemand.
//
// Rules:
//   - rules/analysis/parameter_usage_analysis.md — "Large value to reference"
//   - rules/analysis/parameter_usage_analysis.md — "Semantic demand and recommendation policy are separate"
//   - rules/analysis/parameter_usage_analysis.md — "ResolvedLayout as cost input"
func (a *Analyzer) emitLargeValueParameterAdvisory(parameter ParameterUsageParameterSummary) {
	typ := parameter.DeclaredType
	if parameter.DeclaredRef || typ.Kind == ReferenceType || typ.Kind == SliceType || typ.Kind == VoidType || typ.Kind == InvalidType {
		return
	}
	if sharedReferenceRecommendation(parameter).Status != "recommended" {
		return
	}
	help := "Pass the parameter by shared reference when the function does not need to own or copy the whole value."
	if typ.Kind == ArrayType {
		a.addWarningAtTokenWithMetadata(parameter.Declaration, diagnostics.LargeValueParameter, help, "parameter %q passes large array %s by value; consider ref %s", parameter.Name, typeDisplayName(typ), typeDisplayName(typ))
		return
	}
	a.addWarningAtTokenWithMetadata(parameter.Declaration, diagnostics.LargeValueParameter, help, "parameter %q passes large value %s by value; consider ref %s", parameter.Name, typeDisplayName(typ), typeDisplayName(typ))
}
