package main

import (
	"strings"
	"testing"
)

// The editor outline lists `free` as the type's lifecycle member, its keyword
// keeps keyword highlighting, and Sema's free-specific diagnostics reach the
// shared LSP diagnostic path.
//
// Rules:
//   - rules/declarations/impl.md — §19 "`free`"
//   - rules/memory/destruction.md — §33.3 "LSP"
//   - rules/tooling/lsp.md — "Document symbols"
func TestLSPPresentsCustomFreeLifecycleMember(t *testing.T) {
	source := "module main\n\n" +
		"type Handle struct {\n" +
		"    raw: int,\n" +
		"}\n\n" +
		"impl Handle {\n" +
		"    free {\n" +
		"        defer {\n" +
		"        }\n" +
		"    }\n" +
		"}\n"

	tokens := decodeSemanticTokens(semanticTokensForSource("", source))
	assertSemanticToken(t, tokens, 7, 4, len("free"), "keyword")

	symbols := documentSymbolsForSource("", source)
	var impl *documentSymbol
	for index := range symbols {
		if symbols[index].Name == "impl Handle" {
			impl = &symbols[index]
		}
	}
	if impl == nil {
		t.Fatal("missing impl outline symbol")
	}
	assertDocumentSymbolNames(t, impl.Children, []string{"free"})
	assertDocumentSymbolRangesContainSelections(t, symbols)

	diagnostics := analyze("file:///tmp/sec-lsp-custom-free/main.sec", source)
	found := false
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "defer is not allowed inside free") {
			found = true
		}
	}
	if !found {
		t.Fatalf("LSP diagnostics do not report defer inside free: %+v", diagnostics)
	}
}
