package main

import (
	"os"
	"strings"
	"testing"
)

// Editor signature help preserves inherited overload sets, while conformance
// diagnostics navigate to the specific failed requirement and implementation.
// Rules: rules/declarations/interfaces.md §§5–6,12;
// rules/tooling/lsp.md — signature help and diagnostics.
func TestInterfaceOverloadMatrixLSP(t *testing.T) {
	for _, name := range []string{"valid", "contracts_invalid", "declarations_invalid"} {
		data, err := os.ReadFile("../../testdata/sema/interface_overload_matrix/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		uri := "file:///tmp/sec-interface-overloads/" + name + ".sec"
		errors := 0
		for _, d := range analyze(uri, source) {
			if d.Severity != 1 {
				continue
			}
			errors++
			if name == "contracts_invalid" && len(d.RelatedInformation) == 0 {
				t.Fatal(d)
			}
		}
		want := 3
		if name == "valid" {
			want = 0
		}
		if name == "contracts_invalid" {
			want = 2
		}
		if errors != want {
			t.Fatalf("%s: errors %d want %d", name, errors, want)
		}
		if name == "valid" {
			offset := strings.Index(source, "value.Convert(true)") + len("value.Convert(")
			help, ok := signatureHelpForSource(uri, source, offsetPosition(source, offset))
			if !ok || len(help.Signatures) != 1 || help.Signatures[0].Label != "fn Convert(value: bool) bool" {
				t.Fatalf("wrong selected inherited overload: %+v", help)
			}
		}
	}
}
