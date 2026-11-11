package main

import (
	"sec/internal/lexer"
	"sec/internal/sema"
)

// addRawPointers presents existing source-local proofs with the same
// description as hover; unknown pointer obligations remain explicitly unknown.
// Rules: rules/memory/raw_pointers.md §35(4–6);
// rules/compiler/compiler_analysis.md §18(2,6).
func (r *analyseReport) addRawPointers(analyzer *sema.Analyzer, inSelection func(lexer.Token) bool) {
	for _, fact := range analyzer.RawPointerFacts() {
		if inSelection(fact.Source) {
			r.section("raw pointers").add("%s at %s", fact.Display(), analysePosition(fact.Source))
		}
	}
}
