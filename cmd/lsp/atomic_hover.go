package main

import (
	"strings"

	"sec/internal/sema"
)

// atomicTypeHover presents the resolved core family and its exact public API.
// It uses compiler facts, including template-only applicability documentation,
// rather than reinterpreting the contained type or inventing storage fields.
// Rules: rules/concurrency/atomics.md — §§7(5–8),12–13,43(3–5).
func atomicTypeHover(rng lspRange, typ sema.Type) (hoverResult, bool) {
	fact, ok := sema.AtomicTypeFactsOf(typ)
	if !ok {
		return hoverResult{}, false
	}
	name := lspTypeName(fact.Type)
	if fact.Template {
		name += genericHeaderDisplay(fact.Type.GenericParameters, fact.Type.GenericConstraints)
	}
	return hoverResult{Contents: markupContent{Kind: "markdown", Value: "```sec\n" + name + "\n```" + atomicFactsHoverSuffix(fact)}, Range: rng}, true
}

// atomicFactsHoverSuffix renders compiler-owned signatures for bindings and
// type uses alike. Conditional generic availability stays in compiler metadata.
// Rules: rules/concurrency/atomics.md — §§4(6),7(8),43(3–5).
func atomicFactsHoverSuffix(fact sema.AtomicTypeFacts) string {
	var contents strings.Builder
	contents.WriteString("\n\n" + fact.Documentation + "\n\n```sec\n")
	for _, member := range fact.Members {
		if fact.Template {
			contents.WriteString("// " + member.Documentation + "\n")
		}
		contents.WriteString(member.Signature + "\n\n")
	}
	contents.WriteString("```")
	return contents.String()
}
