package main

import (
	"fmt"
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// lifetimeHover presents the lifetime of the reference holder declared at
// the cursor: its origin and storage, the call whose returned-reference
// summary produced it, the Arena incarnation it depends on and the later
// Reset/Release operations that invalidate it, and the resolved end of the
// borrow the holder retains. It reads ResolvedLifetimeAt only; there is no
// second LSP lifetime engine, and no edit is offered.
// Rules: rules/memory/lifetime_analysis.md — §§ 2, 6, 12; rules/memory/arena.md — § 44;
// rules/tooling/lsp.md — "Hover".
func lifetimeHover(analyzer *sema.Analyzer, token lexer.Token) string {
	if analyzer == nil {
		return ""
	}
	fact, ok := analyzer.ResolvedLifetimeAt(token.File, token.Line, token.Column)
	if !ok {
		return ""
	}
	lines := []string{}
	if fact.ReturnedBy != "" {
		source := "an origin the callee summary leaves unknown"
		if len(fact.ReturnedFrom) > 0 {
			source = "`" + strings.Join(fact.ReturnedFrom, "` or `") + "`"
		}
		lines = append(lines, fmt.Sprintf("- returned reference: `%s` returns a reference into %s", fact.ReturnedBy, source))
	}
	if fact.Origin != "" && !strings.HasPrefix(fact.Origin, "$") {
		origin := fmt.Sprintf("- origin: `%s`", fact.Origin)
		if fact.OriginToken.Line > 0 {
			origin += fmt.Sprintf(" (%d:%d)", fact.OriginToken.Line, fact.OriginToken.Column)
		}
		if fact.Storage != "" {
			origin += fmt.Sprintf(", %s storage", fact.Storage)
		}
		lines = append(lines, origin)
	}
	if fact.ArenaDomain != "" {
		lines = append(lines, fmt.Sprintf("- generation: valid in Arena epoch %d of its domain", fact.Generation))
		if len(fact.Invalidations) == 0 {
			lines = append(lines, "- invalidation: no later Reset or Release of that Arena in this analysis")
		} else {
			sites := []string{}
			for _, invalidation := range fact.Invalidations {
				sites = append(sites, fmt.Sprintf("%s at %d:%d", invalidation.Kind, invalidation.Source.Line, invalidation.Source.Column))
			}
			lines = append(lines, "- invalidation: ends the reference's validity at "+strings.Join(sites, ", ")+"; a later use is rejected")
		}
	}
	lines = append(lines, fmt.Sprintf("- retained by: `%s`; %s", fact.Holder, lifetimeEndSpelling(fact.Ends)))
	return "\n\n**Lifetime**\n\n" + strings.Join(lines, "\n")
}

func lifetimeEndSpelling(ends []sema.ReferenceLifetimeEnd) string {
	text := borrowEndSpelling(ends)
	if text == "when the holder goes out of scope" {
		return "the dependency ends when the holder goes out of scope"
	}
	return "the dependency ends " + text
}
