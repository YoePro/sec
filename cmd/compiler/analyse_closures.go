package main

import (
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// addClosures presents canonical Sema creation/capture/callable-flow facts.
// Facts are verbose insight, not invented advisories or allocation/ABI claims.
// Open/unknown target sets retain their precision and are never presented as
// closed. Safety diagnostics remain mandatory in every analysis selection.
// Rules: rules/analysis/closure_analysis.md — "Diagnostics", "sec analyse",
// "Callable target sets"; rules/tooling/cli.md — § 5.2–§ 5.4.
func (r *analyseReport) addClosures(analyzer *sema.Analyzer, inSelection func(lexer.Token) bool) {
	section := r.section("closures")
	for _, creation := range analyzer.ClosureCreations() {
		if !inSelection(creation.Source) {
			continue
		}
		section.add("creation at %s: body %s, environment=%t, captures=%d, dependencies-known=%t",
			analysePosition(creation.Source), creation.Body, creation.HasEnvironment, len(creation.Captures), creation.DependenciesKnown)
		for _, capture := range creation.Captures {
			section.add("  capture %s: mode=%s, transfer=%s, source at %s, source-unavailable=%t",
				capture.Name, capture.Mode, capture.Transfer, analysePosition(capture.Source), capture.SourceBecomesUnavailable)
		}
	}
	graph := analyzer.CallGraph()
	for _, node := range graph.Nodes() {
		for _, site := range graph.Outgoing(node.ID) {
			if !inSelection(site.Source) || (site.Dispatch != sema.CallDispatchClosure && site.Dispatch != sema.CallDispatchFunctionValue) {
				continue
			}
			precision := "unknown"
			if site.TargetSet.IsClosed {
				precision = "closed"
			} else if site.TargetSet.HasOpenContract {
				precision = "open-contract"
			}
			targets := make([]string, 0, len(site.TargetSet.KnownTargets))
			for _, target := range site.TargetSet.KnownTargets {
				targets = append(targets, string(target))
			}
			section.add("call at %s: %s targets=[%s]", analysePosition(site.Source), precision, strings.Join(targets, ", "))
		}
	}
}
