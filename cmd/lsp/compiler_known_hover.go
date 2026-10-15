package main

import (
	"fmt"
	"sec/internal/sema"
	"strings"
)

// compilerKnownMemberHover presents the shared receiver-resolved registry.
// Rules: rules/compiler/compiler_known_members.md — "LSP", "Synthetic definitions";
// rules/memory/raw_pointers.md — §35(2–6).
func compilerKnownMemberHover(sourceRange lspRange, member sema.CompilerKnownMember) hoverResult {
	kind := string(member.Kind)
	if member.Unsafe {
		kind = "unsafe " + kind
	}
	result := lspTypeName(member.Result)
	if member.Result.Kind == sema.InvalidType || member.Result.Kind == "" {
		result = "context-dependent"
	}
	effects := ""
	if len(member.Effects) > 0 {
		names := make([]string, 0, len(member.Effects))
		for _, effect := range member.Effects {
			names = append(names, string(effect))
		}
		effects = "\n\nEffects: `" + strings.Join(names, "`, `") + "`."
	}
	declaration := fmt.Sprintf("%s %s: %s", kind, member.Name, result)
	resolved := ""
	if member.Signature != "" {
		declaration = member.Signature
		// The registry signature is generic; show the receiver-resolved
		// result at this use as well.
		if result != "context-dependent" {
			resolved = "\n\nResult here: `" + result + "`."
		}
	}
	if member.Rule != "" {
		resolved += "\n\nRule: " + member.Rule + "."
	}
	documentation := ""
	if member.Documentation != "" {
		documentation = "\n\n" + member.Documentation
	}
	// rules/corrections/applied/compiler-known-fundamentals-cross-rulebook-correction-20260907.md
	// § 24(4): hover identifies the member's semantic-authority category.
	category := ""
	switch member.Category {
	case sema.CompilerProvidedFallbackMember:
		category = "\n\nCompiler-provided fallback: an exact user-defined replacement on a user-owned type takes precedence."
	case sema.AuthoritativeCompilerSemanticProperty:
		category = "\n\nAuthoritative compiler property: user code cannot replace it."
	case sema.CompilerKnownOperation:
		category = "\n\nCompiler-known operation."
	case sema.PrivilegedCoreMember:
		category = "\n\nPrivileged core member."
	}
	contents := fmt.Sprintf("```sec\n%s\n```\n\nCompiler-known `%s`.%s%s%s%s", declaration, member.ID, resolved, category, effects, documentation)
	contents += compilerKnownCallerObligations(member)
	return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: sourceRange}
}

// compilerKnownCallerObligations labels required caller proofs separately from
// proven source facts. The same text is used by hover and signature help.
// Rules: rules/memory/raw_pointers.md — §§5.2(1–3),35(2–6).
func compilerKnownCallerObligations(member sema.CompilerKnownMember) string {
	var text strings.Builder
	if member.Unsafe {
		text.WriteString("\n\nRequires an `unsafe` context.")
	}
	if len(member.CallerObligations) > 0 {
		text.WriteString("\n\nCaller obligations (unsafe does not prove these):\n")
		for _, obligation := range member.CallerObligations {
			text.WriteString("\n- " + obligation)
		}
	}
	return text.String()
}
