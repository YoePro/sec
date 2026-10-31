package main

import (
	"os"
	"strings"
	"testing"
)

// Hover/signature help consume exclusive receiving modes resolved by Sema,
// rather than reconstructing mutable pack inference in the editor.
// Rules: rules/declarations/functions.md §38A(3),(10)(3); rules/tooling/lsp.md — "Signature help".
func TestMutableTypePackTooling(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/mutable_calls.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if errors := analyze("", source); len(errors) != 0 {
		t.Fatal(errors)
	}
	at := strings.Index(source, "Count(first, second)")
	if at < 0 {
		t.Fatal("missing call")
	}
	hover, known := hoverForSource("", source, offsetPosition(source, at+1))
	if !known || !strings.Contains(hover.Contents.Value, "Types = (int, bool)") || !strings.Contains(hover.Contents.Value, "(ref mut int, ref mut bool)") {
		t.Fatal(hover, known)
	}
	help, known := signatureHelpForSource("", source, offsetPosition(source, at+len("Count(first, ")))
	if !known || help.ActiveParameter != 1 || len(help.Signatures) != 1 || !strings.Contains(help.Signatures[0].Label, "values[0]: ref mut int, values[1]: ref mut bool") {
		t.Fatal(help, known)
	}
}
