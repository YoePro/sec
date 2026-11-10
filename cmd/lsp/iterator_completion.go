package main

import "sec/internal/sema"

// iteratorBindingCompletion retains ordinary value-type completion while
// presenting Sema's loop resolution for that exact binding identity. Same-name
// locals in other scopes cannot acquire another loop's conformance or Next.
// Generic constraints and borrowed-view runtime requirements are rendered by
// the shared compiler fact, never resolved again in the LSP.
// Rules: rules/control-flow/flowcontrol_for.md — §§37–37.1;
// rules/compiler/semantic_ir.md — §109(1–4),§115(2);
// rules/tooling/lsp.md — §4.1(5–7).
func iteratorBindingCompletion(analyzer *sema.Analyzer, binding sema.ResolvedBinding) completionItem {
	item := completionItem{Label: binding.Name, Kind: 6, Detail: lspTypeName(binding.Type)}
	if binding.ID == 0 {
		return item
	}
	for _, fact := range analyzer.IteratorResolutions() {
		if fact.BindingID == binding.ID {
			item.Documentation = fact.Display()
			break
		}
	}
	return item
}
