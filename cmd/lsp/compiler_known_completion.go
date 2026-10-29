package main

import "sec/internal/sema"

// compilerKnownCompletionItem projects an already eligible member into ordinary
// completion. Missing source names are omitted instead of inserting transport
// identifiers. Shared documentation retains the same facts as hover/signatures.
// Rules: rules/compiler/compiler_known_members.md — "LSP", "User-facing symbol names";
// rules/tooling/lsp.md — "Completion".
func compilerKnownCompletionItem(member sema.CompilerKnownMember) (completionItem, bool) {
	facts := member.DisplayFacts()
	if facts.Name == "" {
		return completionItem{}, false
	}
	kind := 2
	if member.Kind == sema.CompilerKnownProperty {
		kind = 10
	}
	detail := facts.ResultType
	if detail == "context-dependent" {
		detail = facts.Kind
	}
	if member.Unsafe {
		detail = "unsafe " + detail
	}
	if len(member.Effects) > 0 {
		detail += " (effectful)"
	}
	return completionItem{Label: facts.Name, Kind: kind, Detail: detail,
		Documentation: compilerKnownMemberHover(lspRange{}, member).Contents.Value}, true
}

// compilerKnownSignatureLabel consumes the canonical display signature without
// fabricating parameters when the registry does not provide a complete one.
// Rules: rules/compiler/compiler_known_members.md — "LSP", "User-facing symbol names".
func compilerKnownSignatureLabel(member sema.CompilerKnownMember) string {
	facts := member.DisplayFacts()
	if facts.Signature != "" {
		return facts.Signature
	}
	return facts.Label + " (signature unavailable)"
}
