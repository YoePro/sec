package main

import (
	"fmt"
	"strings"

	"sec/internal/lexer"
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

// atomicCapabilityHoverSuffix shows the MD-095 target capability of one
// atomic operation or fence per execution context. An open obligation is
// presented as unproven with its missing fact, never as executable validity
// or a proven unsupported capability; supported names only the plan's
// evidence, since lowering is a separate step.
// Rules: rules/concurrency/atomics.md — §§ 37A(12), 48(4); rules/tooling/diagnostics.md — § 24(4l).
func atomicCapabilityHoverSuffix(analyzer *sema.Analyzer, token lexer.Token) string {
	var lines []string
	for _, result := range analyzer.AtomicCapabilities() {
		if result.Source.File != token.File || result.Source.Line != token.Line || result.Source.Column != token.Column {
			continue
		}
		key := result.Requirement.Key
		context := string(key.Context)
		if context == "" {
			context = "unresolved context"
		}
		if key.NoBlocking {
			context += ", no-blocking"
		}
		if key.InterruptSafe {
			context += ", interrupt-safe"
		}
		switch result.Availability {
		case "supported":
			lines = append(lines, fmt.Sprintf("- %s: supported by the selected plan (%s)", context, result.Capability.Implementation.Identity))
		case "unsupported":
			lines = append(lines, fmt.Sprintf("- %s: not supported by the selected target/profile", context))
		default:
			lines = append(lines, fmt.Sprintf("- %s: unproven, %s", context, result.Missing))
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\n**Target capability**\n" + strings.Join(lines, "\n")
}
