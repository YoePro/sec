package main

import (
	"os"
	"strings"
	"testing"
)

// Method call tooling projects the selected concrete pack and receiving modes,
// keeping the implicit receiver separate from source value parameters.
// Rules: rules/declarations/functions.md §§22,38A(10)(3);
// rules/tooling/lsp.md — "Hover", "Signature help".
func TestTypePackMethodTooling(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/method_calls.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if errors := analyze("", source); len(errors) != 0 {
		t.Fatal(errors)
	}
	at := strings.Index(source, "counter.Mutable(value)")
	if at < 0 {
		t.Fatal("missing call")
	}
	hover, known := hoverForSource("", source, offsetPosition(source, at+len("counter.")+1))
	if !known || !strings.Contains(hover.Contents.Value, "Types = (int)") || !strings.Contains(hover.Contents.Value, "(ref mut int)") {
		t.Fatal(hover, known)
	}
	help, known := signatureHelpForSource("", source, offsetPosition(source, at+len("counter.Mutable(")))
	if !known || len(help.Signatures) != 1 || !strings.Contains(help.Signatures[0].Label, "Counter.Mutable(values[0]: ref mut int)") {
		t.Fatal(help, known)
	}
}
