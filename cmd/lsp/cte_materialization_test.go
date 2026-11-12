package main

import (
	"os"
	"strings"
	"testing"
)

// TestCTEMaterializationGraphLSP presents sanitized typed CTE values while
// retaining nominal and active-payload identities from the compiler service.
// Rules: rules/compiler/compile_time_evaluation.md — §§11,32,36,73;
// rules/tooling/lsp.md — Hover, One semantic source of truth.
func TestCTEMaterializationGraphLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/cte_materialization_graph_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///cte-materialization-graph/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	for _, tc := range []struct{ marker, want string }{
		{":= Nested()", "Result[Option[Point], AppError]"},
		{":= Fields()", "Choice"},
		{":= Nominal()", "Concrete"},
		{":= NoValue()", "None"},
	} {
		offset := strings.LastIndex(source, tc.marker) + len(":= ")
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, tc.want) || !strings.Contains(hover.Contents.Value, "Compile-time value") {
			t.Fatal(tc.marker, hover, ok)
		}
	}
}
