package main

import (
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// compileTimeValueHover presents the value the compiler established for a
// required compile-time position (contract argument, explicit default,
// static initializer) whose root expression starts at the cursor, or why the
// evaluation failed with its executed call chain. It reads the compiler's CTE
// outcomes only and never evaluates in the editor.
// Rules: rules/compiler/compile_time_evaluation.md — § 73; rules/tooling/lsp.md — "Hover".
func compileTimeValueHover(analyzer *sema.Analyzer, token lexer.Token) string {
	if analyzer == nil {
		return ""
	}
	fact, ok := analyzer.CompileTimeValueAt(token.File, token.Line, token.Column)
	if !ok {
		return ""
	}
	if fact.Failure == "" {
		return "\n\n**Compile-time value**\n\n- `" + fact.Root + "` = `" + fact.Value + "`, established by semantic compile-time evaluation"
	}
	line := "- `" + fact.Root + "`: " + fact.Failure + ": " + fact.Reason
	if len(fact.CallChain) > 0 {
		line += "\n- executed: " + strings.Join(fact.CallChain, " -> ")
	}
	return "\n\n**Compile-time evaluation**\n\n" + line
}
