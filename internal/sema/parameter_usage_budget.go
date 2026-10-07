package sema

import (
	"fmt"
	"sort"
)

// ParameterUsageBudget bounds demand refinement, optional recommendations and
// summary transport independently. Zero iterations uses the finite lattice
// bound; zero optional work/depth/bytes disables that operation. These limits
// never disable source validation or turn missing evidence into positive proof.
// Rules: rules/compiler/compiler_analysis.md — §§14(2–3), 15(1–5);
// rules/analysis/parameter_usage_analysis.md — "Analysis budgets",
// "Interactive analysis", "Standard analysis", "Deep analysis".
type ParameterUsageBudget struct {
	MaxSummaryIterations  int
	MaxRecommendationWork int
	MaxTypeDepth          int
	MaxPersistenceRecords int
	MaxPersistenceBytes   int
}

// parameterUsageBudget assigns larger finite budgets for deeper analysis;
// demand semantics and capability requirements are identical at every depth.
// Rules: rules/analysis/parameter_usage_analysis.md — "Analysis budgets",
// "Interactive analysis", "Standard analysis", "Deep analysis".
func parameterUsageBudget(depth AnalysisDepth) ParameterUsageBudget {
	switch depth {
	case AnalysisInteractive:
		return ParameterUsageBudget{4, 4096, 64, 4096, 512 * 1024}
	case AnalysisDeep:
		return ParameterUsageBudget{256, 262144, 256, 262144, 64 * 1024 * 1024}
	default:
		return ParameterUsageBudget{64, 32768, 128, 32768, 8 * 1024 * 1024}
	}
}

// SetParameterUsageBudget configures the next analysis/metadata request.
// Negative values leave the previous policy intact. Existing detached analysis
// snapshots keep their own policy, and changing limits discards pending imports
// so metadata admitted under an earlier policy cannot bypass the new limits.
// Rules: rules/compiler/compiler_analysis.md — §15(1–5);
// rules/analysis/parameter_usage_analysis.md — "Analysis budgets", "Summary invalidation".
func (a *Analyzer) SetParameterUsageBudget(budget ParameterUsageBudget) error {
	if budget.MaxSummaryIterations < 0 || budget.MaxRecommendationWork < 0 || budget.MaxTypeDepth < 0 || budget.MaxPersistenceRecords < 0 || budget.MaxPersistenceBytes < 0 {
		return fmt.Errorf("parameter usage limits must be nonnegative")
	}
	a.parameterBudget = budget
	a.importedParameterDemands = nil
	return nil
}

// ParameterUsageBudget returns a detached configuration value.
// Rules: rules/analysis/parameter_usage_analysis.md — "Analysis budgets".
func (a *Analyzer) ParameterUsageBudget() ParameterUsageBudget { return a.parameterBudget }

// ParameterRecommendationCoverage reports optional work, not source validity,
// demand precision or proof of absence of a narrowing opportunity.
// Rules: rules/compiler/compiler_analysis.md — §15(5);
// rules/analysis/parameter_usage_analysis.md — "Interactive analysis", "Deep analysis".
type ParameterRecommendationCoverage struct {
	MaxWork, MaxTypeDepth                  int
	VisitedWork                            int
	EvaluatedCandidates, SkippedCandidates int
}

// ParameterUsageBudgetError means optional summary transport did not finish.
// No positive or partially validated artifact is published on this error.
// Rules: rules/compiler/compiler_analysis.md — §§14(3), 15(3–5);
// rules/analysis/parameter_usage_analysis.md — "Persisted summary versioning".
type ParameterUsageBudgetError struct{ Operation, Limit string }

// Error explains the transport limit without labeling source code invalid.
// Rules: rules/compiler/compiler_analysis.md — §§7(6–8), 15(5).
func (e *ParameterUsageBudgetError) Error() string {
	return "parameter demand " + e.Operation + " budget exhausted: " + e.Limit
}

type parameterWorkBudget struct{ remaining, visited, maxDepth int }

// take admits one semantic work record without integer overflow or wall-clock
// dependence. Failed complete-unit admission never enables a partial proof.
// Rules: rules/compiler/compiler_analysis.md — §§14(2–3), 15(2–5).
func (b *parameterWorkBudget) take(count int) bool {
	if count < 0 || count > b.remaining {
		b.visited += b.remaining
		b.remaining = 0
		return false
	}
	b.remaining -= count
	b.visited += count
	return true
}

// typeFits preflights the recursive type structures used by recommendation
// capability/cost helpers and persisted signature identity. Recommendation
// references are storage boundaries; transport also walks callable/reference
// signature components. A repeated pointer on a path is conservatively skipped
// as unsupported instead of letting an optional helper recurse forever.
// Rules: rules/compiler/compiler_analysis.md — §15(2–5);
// rules/analysis/parameter_usage_analysis.md — "ResolvedLayout as cost input",
// "Persisted summary versioning", "Analysis budgets".
func (b *parameterWorkBudget) typeFits(typ Type, depth int, transport bool, active map[*Type]bool) bool {
	if depth > b.maxDepth || !b.take(1) {
		return false
	}
	if !transport && (typ.Kind == ReferenceType || typ.Kind == SliceType || typ.Kind == RawPtrType || typ.Kind == FunctionType) {
		return true
	}
	pointer := func(child *Type) bool {
		if child == nil {
			return true
		}
		if active[child] {
			return false
		}
		active[child] = true
		ok := b.typeFits(*child, depth+1, transport, active)
		delete(active, child)
		return ok
	}
	if !pointer(typ.Element) || !pointer(typ.FunctionReturnType) {
		return false
	}
	for _, child := range typ.TypeArgs {
		if !b.typeFits(child, depth+1, transport, active) {
			return false
		}
	}
	for _, child := range typ.FunctionParameterTypes {
		if !b.typeFits(child, depth+1, transport, active) {
			return false
		}
	}
	for _, field := range typ.Fields {
		if !b.typeFits(field.Type, depth+1, transport, active) {
			return false
		}
	}
	for _, variant := range typ.UnionVariants {
		if !b.take(1) || !pointer(variant.Payload) {
			return false
		}
		for _, field := range variant.PayloadFields {
			if !b.typeFits(field.Type, depth+1, transport, active) {
				return false
			}
		}
	}
	return true
}

// persistedEntryFits charges validation of a complete symbolic summary,
// counting independent demand and projection records before they are trusted.
// Rules: rules/analysis/parameter_usage_analysis.md — "Separate compilation",
// "Analysis budgets", "Persisted summary versioning".
func (b *parameterWorkBudget) persistedEntryFits(entry persistedParameterDemand) bool {
	if !b.take(1) {
		return false
	}
	parameterFits := func(parameter persistedParameter) bool {
		if !b.take(1) || !b.take(len(parameter.Demand.Shapes)) || !b.take(len(parameter.Demand.Storage)) {
			return false
		}
		for _, path := range parameter.Projections {
			if !b.take(1) || !b.take(len(path)) {
				return false
			}
		}
		return true
	}
	for _, parameter := range entry.Parameters {
		if !parameterFits(parameter) {
			return false
		}
	}
	return entry.Receiver == nil || parameterFits(*entry.Receiver)
}

// summaryFits preflights complete export units before copying source-local
// demand/access structures or deriving recursive signature identities.
// Rules: rules/compiler/compiler_analysis.md — §15(2–5);
// rules/analysis/parameter_usage_analysis.md — "Function summaries", "Analysis budgets".
func (b *parameterWorkBudget) summaryFits(summary *ParameterUsageCallableSummary) bool {
	if !b.take(1) {
		return false
	}
	parameterFits := func(parameter ParameterUsageParameterSummary) bool {
		if !b.take(1) || !b.take(len(parameter.Demand.Shapes)) || !b.take(len(parameter.Demand.Storage)) || !b.typeFits(parameter.DeclaredType, 1, true, map[*Type]bool{}) {
			return false
		}
		for _, use := range parameter.Uses {
			if !b.take(1) || !b.take(len(use.Place.Projections)) {
				return false
			}
		}
		return true
	}
	for _, parameter := range summary.Parameters {
		if !parameterFits(parameter) {
			return false
		}
	}
	return summary.Receiver == nil || parameterFits(*summary.Receiver)
}

// ParameterImportCoverage reports signature work on admitted metadata. A
// skipped signature remains a missing summary, never positive demand evidence.
// Rules: rules/compiler/compiler_analysis.md — §15(3–5);
// rules/analysis/parameter_usage_analysis.md — "Separate compilation", "Analysis budgets".
type ParameterImportCoverage struct {
	MaxWork, MaxTypeDepth, VisitedWork   int
	InstalledCallables, SkippedCallables int
}

// ImportCoverage returns detached metadata for this completed analysis request.
// Rules: rules/analysis/parameter_usage_analysis.md — "Analysis budgets", "Function summaries".
func (p *ParameterUsageAnalysis) ImportCoverage() ParameterImportCoverage {
	if p == nil {
		return ParameterImportCoverage{}
	}
	return p.importCoverage
}

// orderedRecommendationSummaries admits candidates by canonical source order
// with callable identity as a tie breaker, independent of registration/map order.
// Rules: rules/analysis/parameter_usage_analysis.md — "Determinism", "Analysis budgets".
func (p *ParameterUsageAnalysis) orderedRecommendationSummaries() []*ParameterUsageCallableSummary {
	summaries := make([]*ParameterUsageCallableSummary, 0, len(p.summaryOrder))
	for _, id := range p.summaryOrder {
		if summary := p.summaries[id]; summary != nil {
			summaries = append(summaries, summary)
		}
	}
	sort.Slice(summaries, func(i, j int) bool {
		left, right := summaries[i], summaries[j]
		if left.Declaration.File != right.Declaration.File {
			return left.Declaration.File < right.Declaration.File
		}
		if left.Declaration.Line != right.Declaration.Line {
			return left.Declaration.Line < right.Declaration.Line
		}
		if left.Declaration.Column != right.Declaration.Column {
			return left.Declaration.Column < right.Declaration.Column
		}
		return left.Callable < right.Callable
	})
	return summaries
}
