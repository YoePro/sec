package main

import (
	"sec/internal/lexer"
	"sec/internal/sema"
)

// addSchedulingPoints presents compiler-resolved scheduling properties without
// classifying function names or manufacturing runtime progress/safety proofs.
// Rules: rules/concurrency/scheduling.md §§15,50,55; rules/tooling/cli.md §5.4.
func (r *analyseReport) addSchedulingPoints(analyzer *sema.Analyzer, inSelection func(lexer.Token) bool) {
	section := r.section("scheduling points")
	for _, fact := range analyzer.SchedulingPoints() {
		if !inSelection(fact.Source) {
			continue
		}
		if fact.TaskSchedulingPoint && fact.CurrentCancellationPoint {
			section.add("logical task scheduling and cancellation point at %s", analysePosition(fact.Source))
		} else if fact.PhysicalSchedulerHint {
			section.add("physical thread scheduler hint at %s (not a task scheduling or cancellation point)", analysePosition(fact.Source))
		}
	}
}
