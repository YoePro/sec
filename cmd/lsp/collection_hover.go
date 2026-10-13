package main

import "sec/internal/sema"

// collectionFactsHoverSuffix presents Sema's detached collection metadata.
// Rules: rules/tooling/lsp.md — "One semantic source of truth", "Hover".
func collectionFactsHoverSuffix(typ sema.Type) string {
	facts, ok := sema.CollectionFactsOf(typ)
	if !ok {
		return ""
	}
	return "\n\n```text\n" + facts.Display() + "\n```"
}
