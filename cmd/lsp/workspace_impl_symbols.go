package main

import "strings"

// workspaceImplExtensionEntry projects the existing document-symbol header into
// the per-file workspace index. This is a source inventory, not a second impl
// validator. The target's strongest qualified visibility applies to the header;
// the display prefix "impl extends" cannot make a hidden target public.
// Rules: rules/declarations/impl.md — §§4–5;
// rules/tooling/lsp.md — "Workspace symbols";
// rules/foundations/names_scopes_visibility.md — §12.
func workspaceImplExtensionEntry(symbol documentSymbol, target, module string) workspaceSymbolEntry {
	entry := workspaceSymbolEntry{name: symbol.Name, kind: symbol.Kind, container: module, rng: symbol.SelectionRange}
	for _, component := range strings.Split(target, ".") {
		if prefix := visibilityPrefix(component); len(prefix) > len(entry.accessPrefix) {
			entry.accessPrefix = prefix
		}
	}
	return entry
}

// visibilityScope shares visibility filtering between workspace lookup and lazy
// resolve, including header symbols whose display name begins with a keyword.
// Rules: rules/tooling/lsp.md — "Workspace symbols";
// rules/foundations/names_scopes_visibility.md — §12.
func (entry workspaceSymbolEntry) visibilityScope() string {
	if entry.accessPrefix != "" {
		return entry.accessPrefix
	}
	return visibilityPrefix(entry.name)
}
