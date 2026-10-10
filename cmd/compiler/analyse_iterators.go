package main

import (
	"sec/internal/lexer"
	"sec/internal/sema"
)

// addIterators presents snapshot-owned conformance and static Next resolution
// in source order, sharing its factual description with editor hover.
// Rules: rules/compiler/semantic_ir.md — §109(1–4);
// rules/compiler/compiler_analysis.md — §18(6),§61(2).
func (r *analyseReport) addIterators(analyzer *sema.Analyzer, inSelection func(lexer.Token) bool) {
	section := r.section("iterators")
	for _, fact := range analyzer.IteratorResolutions() {
		if inSelection(fact.Source) {
			section.add("%s at %s", fact.Display(), analysePosition(fact.Source))
		}
	}
}
