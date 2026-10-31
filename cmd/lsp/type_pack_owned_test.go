package main

import (
	"os"
	"strings"
	"testing"
)

// Editor contracts preserve owned receiving types instead of displaying the
// shared-reference modes used by borrowed packs.
// Rules: rules/declarations/functions.md §38A(3),(10)(3); rules/tooling/lsp.md — "Hover", "Signature help".
func TestOwnedTypePackTooling(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/owned_calls.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if errors := analyze("", source); len(errors) != 0 {
		t.Fatal(errors)
	}
	at := strings.Index(source, "Count(value, true)")
	if at < 0 {
		t.Fatal("missing call")
	}
	hover, known := hoverForSource("", source, offsetPosition(source, at+1))
	if !known || !strings.Contains(hover.Contents.Value, "Types = (int, bool)") || !strings.Contains(hover.Contents.Value, "Pack receiving types: `(int, bool)`") {
		t.Fatal(hover, known)
	}
	help, known := signatureHelpForSource("", source, offsetPosition(source, at+len("Count(value, ")))
	if !known || help.ActiveParameter != 1 || len(help.Signatures) != 1 || !strings.Contains(help.Signatures[0].Label, "values[0]: int, values[1]: bool") {
		t.Fatal(help, known)
	}
}
