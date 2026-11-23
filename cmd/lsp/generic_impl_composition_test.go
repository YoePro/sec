package main

import (
	"os"
	"strings"
	"testing"
)

// Signature help consumes concrete compiler-owned lifecycle previews, including
// on failed argument checks, rather than independently inferring owner types.
// Rules: rules/declarations/impl.md §§15–16,21–22;
// rules/declarations/generics.md §§17,25; rules/tooling/lsp.md §13.
func TestGenericImplCompositionLSP(t *testing.T) {
	var source string
	for _, name := range []string{"primary", "extension"} {
		data, err := os.ReadFile("../../testdata/sema/generic_impl_composition/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		if source != "" {
			source += "\n" + strings.Replace(string(data), "module main", "", 1)
		} else {
			source = string(data)
		}
	}
	uri := "file:///tmp/sec-generic-composition/main.sec"
	for _, d := range analyze(uri, source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	for _, tc := range []struct{ call, label string }{{"new Box[int](3)", "init(value: int)"}, {"new Box[bool](true)", "init(value: bool)"}, {"new Fallible[int, Fault](3)", "init(value: int)"}, {"new Fallible[bool, OtherFault](true)", "init(value: bool)"}} {
		offset := strings.Index(source, tc.call) + len(tc.call) - 1
		help, ok := signatureHelpForSource(uri, source, offsetPosition(source, offset))
		if !ok || len(help.Signatures) != 1 || help.Signatures[0].Label != tc.label {
			t.Fatalf("%s: %+v, %v", tc.call, help, ok)
		}
		if strings.Contains(tc.call, "Fallible") {
			want := "Construction error: `Fault`"
			if strings.Contains(tc.call, "OtherFault") {
				want = "Construction error: `OtherFault`"
			}
			if !strings.Contains(help.Signatures[0].Documentation.Value, want) {
				t.Fatalf("lost concrete error channel: %+v", help)
			}
		}
	}
	invalid := strings.Replace(source, "new Box[int](3)", "new Box[int](true)", 1)
	errors := 0
	for _, d := range analyze(uri, invalid) {
		if d.Severity == 1 {
			errors++
			if !strings.Contains(d.Message, "must be int, got bool") {
				t.Fatal(d)
			}
		}
	}
	if errors != 1 {
		t.Fatalf("got %d errors, want one concrete argument mismatch", errors)
	}
}
