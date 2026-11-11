package main

import (
	"fmt"
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// arenaHover presents the Arena at the cursor (its declaration or the receiver
// of an Arena operation): domain, how and where the domain was created,
// backing and provider, capacity, the represented epoch at this point and the
// allocation profile. It reads ResolvedArenaAt only; a domain whose creation
// is not visible here (an Arena parameter) is shown as caller-provided rather
// than given an invented backing.
// Rules: rules/memory/arena.md — §§ 4.2, 10–12, 44, 77; rules/tooling/lsp.md — "Hover".
func arenaHover(analyzer *sema.Analyzer, token lexer.Token) string {
	if analyzer == nil {
		return ""
	}
	fact, ok := analyzer.ResolvedArenaAt(token.File, token.Line, token.Column)
	if !ok {
		return ""
	}
	domain := strings.TrimPrefix(fact.Domain, "$")
	if len(domain) > 25 {
		domain = domain[:25] + "…"
	}
	lines := []string{fmt.Sprintf("- domain: `%s`", domain)}
	profile := "hosted analysis default"
	if fact.Profile != "" {
		profile = "`" + fact.Profile + "`"
	}
	if creation := fact.Creation; creation != nil {
		lines = append(lines, fmt.Sprintf("- created by: `Arena.%s` at %d:%d", creation.Constructor, creation.Source.Line, creation.Source.Column))
		if creation.Constructor == "FromBuffer" {
			lines = append(lines, fmt.Sprintf("- backing: borrowed `%s` for the whole Arena lifetime; no provider is involved", creation.Backing))
		} else {
			kind := "owned backing"
			if creation.Constructor == "Growable" {
				kind = "owned, growable backing (later growth acquires segments from the provider)"
			}
			lines = append(lines, "- backing: "+kind)
			lines = append(lines, "- provider: the selected target/profile backing provider (profile "+profile+")")
		}
		if creation.Capacity != "" {
			capacity := "`" + creation.Capacity + "`"
			if creation.CapacityValue != "" && creation.CapacityValue != creation.Capacity {
				capacity += " = " + creation.CapacityValue
			}
			lines = append(lines, "- capacity: "+capacity+" bytes requested")
		}
	} else {
		lines = append(lines, "- created by: the caller; backing, provider and capacity belong to that domain's creation")
	}
	epoch := fmt.Sprintf("- epoch: %d (Reset calls on the analyzed path before this point)", fact.Epoch)
	if fact.Epoch == 0 {
		epoch = "- epoch: 0 (no Reset before this point on the analyzed path)"
	}
	lines = append(lines, epoch)
	lines = append(lines, "- profile: "+profile)
	return "\n\n**Arena**\n\n" + strings.Join(lines, "\n")
}
