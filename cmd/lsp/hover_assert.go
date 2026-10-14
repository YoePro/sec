package main

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// assertionHover presents the Sema-resolved assertion fact at the assert
// keyword: whether the compiler proves the condition, the registered panic
// reason when it does not, the static message, and the refinement the
// successful assertion provides to later code.
//
// Rules:
//   - rules/errors/panic.md — § 15.6 "Assertion refinement", § 15.8, § 28 "Diagnostics and tooling"
//   - rules/tooling/lsp.md — "Hover"
func assertionHover(text string, program *ast.Program, analyzer *sema.Analyzer, hovered lexer.Token) (hoverResult, bool) {
	for _, node := range astNodesInProgram(program) {
		statement, ok := node.(*ast.AssertStatement)
		if !ok || !sameSourceToken(statement.Token, hovered) {
			continue
		}
		fact, ok := analyzer.ResolvedAssertionOf(statement)
		if !ok {
			return hoverResult{}, false
		}
		lines := []string{"### `assert`", "Condition: `" + fact.Condition.String() + "`"}
		if fact.Proven {
			lines = append(lines, "Proof: `proven` — no runtime check and no panic effect")
		} else {
			reason := string(fact.Reason)
			if definition, ok := diagnostics.PanicReasonByID(fact.ReasonID); ok {
				reason = definition.Name
			}
			lines = append(lines, "Proof: `not proven` — checked at run time; failure panics with `"+reason+"`")
		}
		if fact.HasMessage {
			lines = append(lines, "Message: `"+fact.Message+"`")
		}
		lines = append(lines, "Refinement: following code may rely on `"+fact.Condition.String()+"` until it is invalidated by mutation")
		return hoverResult{
			Contents: markupContent{Kind: "markdown", Value: strings.Join(lines, "\n\n")},
			Range:    tokenRange(text, statement.Token),
		}, true
	}
	return hoverResult{}, false
}
