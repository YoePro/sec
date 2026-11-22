package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOperatorOperandDiagnosticsLSP shares the compiler ID, operator range and
// repair guidance rather than recognizing errors by their displayed message.
// Rules: rules/tooling/diagnostics.md — §§8–9,20;
// rules/tooling/lsp.md — Diagnostics.
func TestOperatorOperandDiagnosticsLSP(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/operator_operand_types_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
		count := 0
		for _, d := range analyze(uri, string(data)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code != "S1259" || !strings.Contains(d.Message, "Use operands with compatible types") || d.Range.End.Character != d.Range.Start.Character+1 {
				t.Fatal(d)
			}
		}
		want := 0
		if mode == "invalid" {
			want = 4
		}
		if count != want {
			t.Fatal(mode, count)
		}
	}
}
