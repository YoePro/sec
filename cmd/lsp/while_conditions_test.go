package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Valid effectful bool/try conditions and invalid plain/try integer conditions
// use shared compiler analysis rather than editor-specific truthiness rules.
// Rules: rules/control-flow/flowcontrol_while.md — §§3–7,30;
// rules/tooling/lsp.md — Diagnostics.
func TestWhileConditionDiagnosticsLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, name := range []string{"effects", "non_bool_invalid"} {
		data, err := os.ReadFile("../../testdata/control_flow/while_conditions/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		items := analyze(uri, string(data))
		want := 0
		if name != "effects" {
			want = 2
		}
		if len(items) != want {
			t.Fatal(name, items)
		}
		for _, item := range items {
			if item.Severity != 1 || !strings.Contains(item.Message, "while condition must") || !strings.Contains(item.Message, "bool, got int") {
				t.Fatal(item)
			}
		}
	}
}
