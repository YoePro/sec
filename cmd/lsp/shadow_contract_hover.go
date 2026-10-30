package main

import (
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// shadowContractHover presents the compiler's published shadow-knowledge
// contract of a register impl member at its declaration or call name. It is
// static API information, never runtime state, and a Known shadow value is
// not presented as current hardware state.
// Rules: rules/tooling/lsp.md — "Register shadow knowledge (MD-113)";
// rules/corrections/applied/MD-113-shadow-knowledge-across-calls-decision-20261030.md — § 7(7).
func shadowContractHover(analyzer *sema.Analyzer, token lexer.Token) string {
	_, summary, ok := analyzer.ShadowContractAt(token.File, token.Line, token.Column)
	if !ok {
		return ""
	}
	var parts []string
	for _, entry := range []struct {
		label  string
		fields []string
	}{
		{"Requires known", summary.Requires},
		{"Ensures known", summary.Ensures},
		{"Known when it returns true", summary.EnsuresWhenTrue},
		{"May make unknown", summary.Invalidates},
	} {
		if len(entry.fields) > 0 {
			parts = append(parts, entry.label+": `"+strings.Join(entry.fields, "`, `")+"`.")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n\n**Shadow contract.** " + strings.Join(parts, " ") + " Static compiler information; a known shadow value is software knowledge, not current hardware state."
}
