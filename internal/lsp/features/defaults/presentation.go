package defaults

import "sec/internal/sema"

// CompletionDetail presents the canonical semantic default with the same
// bounded array preview as hover, without expanding defaults into source.
// Rules: rules/types/default_values.md — "LSP"; rules/tooling/lsp.md — completion.
func CompletionDetail(typ sema.Type) string {
	detail := string(typ.Kind)
	if value, _, ok := sema.DefaultValuePreview(typ, 8); ok {
		detail += " = " + value
	}
	return detail
}
