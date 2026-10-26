package sema

import "sort"

// ClosureCreations returns detached creation/capture/target snapshots in stable
// source order. It publishes existing facts without rerunning closure analysis
// or manufacturing facts for rejected creations.
// Rules: rules/analysis/closure_analysis.md — "Callable creation", "Capture record",
// "sec analyse"; rules/compiler/compiler_analysis.md — immutable analysis results.
func (a *Analyzer) ClosureCreations() []ClosureCreationSummary {
	if a == nil {
		return nil
	}
	result := make([]ClosureCreationSummary, 0, len(a.resolvedClosureCreations))
	for lambda := range a.resolvedClosureCreations {
		if summary, ok := a.ResolvedClosureCreationOf(lambda); ok {
			result = append(result, summary)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if left.Source.File != right.Source.File {
			return left.Source.File < right.Source.File
		}
		if left.Source.ByteStart != right.Source.ByteStart {
			return left.Source.ByteStart < right.Source.ByteStart
		}
		return left.CreationSite < right.CreationSite
	})
	return result
}
