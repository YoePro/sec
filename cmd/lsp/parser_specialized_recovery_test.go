package main

import (
	"os"
	"testing"
)

// The editor keeps later function symbols after specialized recovery while
// still publishing the compiler's parser errors for the broken source.
// Rules: rules/compiler/parser_recovery.md — "Interaction with LSP", "Document symbols".
func TestSpecializedParserRecoveryPreservesLSPSymbolsAndErrors(t *testing.T) {
	for _, file := range []string{"specialized_skip_ranges_invalid.sec", "specialized_skip_boundaries_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/parser/" + file)
		if err != nil {
			t.Fatal(err)
		}
		const uri = "file:///tmp/sec-specialized-parser-recovery/main.sec"
		assertDocumentSymbolNames(t, documentSymbolsForSource(uri, string(data)), []string{"After"})
		diagnostics := analyze(uri, string(data))
		if len(diagnostics) == 0 {
			t.Fatalf("%s: invalid fixture accepted", file)
		}
		syntaxError := false
		for _, diagnostic := range diagnostics {
			if diagnostic.Code == "P2013" && diagnostic.Severity == 1 {
				syntaxError = true
			}
		}
		if !syntaxError {
			t.Fatalf("%s lost assignment parser error: %+v", file, diagnostics)
		}
	}
}
