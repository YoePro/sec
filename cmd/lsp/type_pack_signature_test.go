package main

import (
	"os"
	"strings"
	"testing"
)

// Signature help preserves ordered concrete receiving modes and source argument
// selection, including an empty trailing pack and ordinary scalar generics.
// Rules: rules/declarations/functions.md §38A(4),(10)(3);
// rules/tooling/lsp.md — "Signature help".
func TestTypePackSignatureHelp(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/shared_unused.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, test := range []struct {
		marker, want, pack string
		active, count      int
	}{
		{`Printf("hello"`, "fn Printf(format: string) int", "Types = ()", 0, 1},
		{`Printf("values", 1, true, `, "values[2]: ref string", "Types = (int, bool, string)", 3, 4},
		{`Printf("reordered", true, `, "values[0]: ref bool, values[1]: ref int", "Types = (bool, int)", 2, 3},
		{`Printf("references", `, "values[0]: ref int", "Types = (int)", 1, 2},
		{`Printf("reborrow", `, "values[0]: ref int", "Types = (int)", 1, 2},
		{`Select[int](*exclusive, true, `, "head: int, values[0]: ref bool, values[1]: ref string", "Types = (bool, string)", 2, 3},
	} {
		at := strings.Index(source, test.marker)
		if at < 0 {
			t.Fatal(test.marker)
		}
		help, ok := signatureHelpForSource("", source, offsetPosition(source, at+len(test.marker)))
		if !ok || len(help.Signatures) != 1 || help.ActiveParameter != test.active {
			t.Fatalf("%s: found=%v help=%+v", test.marker, ok, help)
		}
		signature := help.Signatures[0]
		if len(signature.Parameters) != test.count || !strings.Contains(signature.Label, test.want) || !strings.Contains(signature.Documentation.Value, test.pack) {
			t.Fatalf("%s: %+v", test.marker, signature)
		}
	}
}

// A selected fixed overload must not acquire help from a competing pack template.
// Rules: rules/declarations/functions.md §38A(4)(5); rules/tooling/lsp.md — "Unknown facts".
func TestTypePackSignatureHelpDoesNotReplaceFixedOverload(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/shared_overloads.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	at := strings.LastIndex(source, "Choose(value)") + len("Choose(")
	if help, ok := signatureHelpForSource("", source, offsetPosition(source, at)); ok {
		t.Fatalf("fixed call acquired pack signature: %+v", help)
	}
}
