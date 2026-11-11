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
// rules/memory/arena.md — §§36,44,119–120 (canonical invalidation blockers).
func allocationOperationHover(analyzer *sema.Analyzer, token lexer.Token) string {
	lifecycle := ""
	for _, blocker := range analyzer.ArenaInvalidationBlockersAt(token.File, token.Line, token.Column) {
		lifecycle += "\n\n**Arena lifecycle blocked**\n\n" + blocker.Error() + "\n\n" + blocker.Help
	}
	lines := []string{}
	for _, fact := range analyzer.AllocationContextFacts() {
		if fact.Source.File == token.File && fact.Source.Line == token.Line && fact.Source.Column == token.Column {
			lines = append(lines, sema.AllocationContextDescription(fact))
		}
	}
	if len(lines) == 0 {
		return lifecycle
	}
	return "\n\n**Allocation context**\n\n" + strings.Join(lines, "\n\n") + lifecycle
}
