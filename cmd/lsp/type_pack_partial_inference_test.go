package main

import (
	"os"
	"strings"
	"testing"
)

// Tooling publishes only completed result/parameter types after argument-only
// overload selection, including a deferred None argument.
// Rules: rules/declarations/generics.md §§18,29;
// rules/declarations/functions.md §38A(10)(3); rules/tooling/lsp.md — "Signature help".
func TestTypePackPartialInferenceTooling(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/overloaded_result_none.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if errors := analyze("", source); len(errors) != 0 {
		t.Fatal(errors)
	}
	at := strings.Index(source, "Context(true, None)")
	if at < 0 {
		t.Fatal("missing call")
	}
	hover, known := hoverForSource("", source, offsetPosition(source, at+1))
	if !known || !strings.Contains(hover.Contents.Value, "Option[int]") {
		t.Fatal(hover, known)
	}
	help, known := signatureHelpForSource("", source, offsetPosition(source, at+len("Context(true, ")))
	if !known || help.ActiveParameter != 1 || len(help.Signatures) != 1 || !strings.Contains(help.Signatures[0].Label, "value: Option[int]") || !strings.HasSuffix(help.Signatures[0].Label, "Option[int]") {
		t.Fatal(help, known)
	}
}
