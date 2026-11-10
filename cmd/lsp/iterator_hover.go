package main

import (
	"sec/internal/lexer"
	"sec/internal/sema"
	"strings"
)

// iteratorHover presents the compiler's resolution at the for keyword, source
// expression head, value binding and its resolved uses. It appends validated
// for-binding modes for all iterable categories. It performs no interface/member lookup.
// Rules: rules/control-flow/flowcontrol_for.md — §§4–10,37–37.1;
// rules/compiler/semantic_ir.md — §109(1–4);
// rules/compiler/compiler_analysis.md — §18(6); rules/tooling/lsp.md — "Hover";
// rules/collections/collections.md — §§14.2,14.6.
func iteratorHover(text string, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
	modes := []string{}
	for _, fact := range analyzer.ForBindingModes() {
		if sameSourceToken(token, fact.Loop) || sameSourceToken(token, fact.Binding) || sameSourceToken(token, fact.ModeToken) {
			modes = append(modes, fact.Display())
		}
	}
	suffix := strings.Join(modes, "\n\n")
	withModes := func(value string) hoverResult {
		if suffix != "" {
			value += "\n\n" + suffix
		}
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: value}, Range: tokenRange(text, token)}
	}

	if fact, known := analyzer.MapIterationAt(token.File, token.Line, token.Column); known && fact.BodyValid {
		return withModes(fact.MapEntries.Display()), true
	}
	binding, bound := analyzer.ResolvedBindingAt(token.File, token.Line, token.Column)
	for _, fact := range analyzer.IteratorResolutions() {
		if sameSourceToken(token, fact.Source) || sameSourceToken(token, fact.Iterable) || sameSourceToken(token, fact.Binding) || (bound && fact.BindingID != 0 && binding.ID == fact.BindingID) {
			return withModes(fact.Display()), true
		}
	}
	if suffix != "" {
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: suffix}, Range: tokenRange(text, token)}, true
	}
	return hoverResult{}, false
}
