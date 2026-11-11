package main

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/diagnostics"
)

// LSP projects the same registered mixed-character errors as compilation;
// equal storage widths never authorize an implicit character conversion.
// Rules: rules/foundations/operators.md §45; rules/compiler/semantic_ir.md §125;
// rules/tooling/lsp.md — shared diagnostic model.
func TestCharacterComparisonMixedTypesLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/character_comparisons_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	values := analyze(uriFromPath(filepath.Join(t.TempDir(), "main.sec")), string(data))
	if len(values) != 8 {
		t.Fatal(values)
	}
	for _, problem := range values {
		if problem.Severity != 1 || (problem.Code != diagnostics.OperatorNonComparable && problem.Code != diagnostics.OperatorNonOrderable) || problem.Range.End.Character <= problem.Range.Start.Character {
			t.Fatal(problem)
		}
	}
}
