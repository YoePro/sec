package main

import "sec/internal/sema"

// sourceVisibleFunctionOverloads projects Sema's file-private function boundary
// into general and expected-return completion, including ordinary core helpers.
// Rules: rules/foundations/names_scopes_visibility.md — §§12.3, 22;
// rules/tooling/lsp.md — Completion.
func sourceVisibleFunctionOverloads(analyzer *sema.Analyzer, functions []sema.Function, sourceFile string) []sema.Function {
	visible := make([]sema.Function, 0, len(functions))
	for _, function := range functions {
		if analyzer.FunctionVisibleFromSource(function, sourceFile) {
			visible = append(visible, function)
		}
	}
	return visible
}
