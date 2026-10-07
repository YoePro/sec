package main

import (
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// allocationOperationHover presents exact operation-point context evidence,
// including operators and compiler-known Arena members with no callable node.
// Matching includes the source file; ordinary syntax/type hover is preserved.
// Rules: rules/memory/allocation.md — §29(1),(3); rules/tooling/lsp.md — "Hover", "Allocation and escape analysis".
func allocationOperationHover(analyzer *sema.Analyzer, token lexer.Token) string {
	lines := []string{}
	for _, fact := range analyzer.AllocationContextFacts() {
		if fact.Source.File == token.File && fact.Source.Line == token.Line && fact.Source.Column == token.Column {
			lines = append(lines, sema.AllocationContextDescription(fact))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\n**Allocation context**\n\n" + strings.Join(lines, "\n\n")
}
