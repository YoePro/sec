package main

import "testing"

// TestCTEMaterializationGraphCLI admits typed semantic values independently
// of whether the later physical lowering supports every aggregate carrier.
// Rules: rules/compiler/compile_time_evaluation.md — §§30–36,52(5–6).
func TestCTEMaterializationGraphCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/cte_materialization_graph_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
}
