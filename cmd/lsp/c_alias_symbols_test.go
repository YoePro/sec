package main

import "testing"

// MD-021 § 3 and lsp.md "Document symbols": a standalone @cAlias is an
// outline symbol named by its c:: spelling, with its target as detail.
func TestDocumentSymbolsIncludeCAliasDeclarations(t *testing.T) {
	source := `module amd64

@cAlias(
    name: "c::stddef::size_t",
    target: c::ulong
)
`
	symbols := documentSymbolsForSource("", source)
	assertDocumentSymbolNames(t, symbols, []string{"amd64", "c::stddef::size_t"})
	if symbols[1].Detail != "C alias of c::ulong" || symbols[1].Kind != 26 || symbols[1].Range.End.Line != 5 {
		t.Fatalf("alias symbol = %+v", symbols[1])
	}
	assertDocumentSymbolRangesContainSelections(t, symbols)
}
