package main

import (
	"os"
	"strings"
	"testing"
)

// Materializers are static associated functions in completion/hover, with
// borrowed source parameters and no instance receiver.
// Rules: rules/corrections/md109-core-string-materialization-correction.md
// — §§2–3,6,9.3,10.2.
func TestStringMaterializationStaticTooling(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/string_materialization_static_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, item := range []struct{ name, element string }{{"FromByteArray", "byte"}, {"FromRuneArray", "rune"}} {
		at := strings.LastIndex(source, "string."+item.name) + len("string.") + 1
		hover, ok := hoverForSource("", source, offsetPosition(source, at))
		want := "static fn " + item.name + "(data: ref " + item.element + "[]) Result[string, StringError]"
		if !ok || !strings.Contains(hover.Contents.Value, want) {
			t.Fatalf("%s hover: %+v, %v; want %s", item.name, hover, ok, want)
		}
	}
	static := "module main\nfn Use() void {\n    string.\n}\n"
	items := completeSource("", static, strings.Index(static, "string.")+len("string."))
	assertCompletionLabels(t, items, []string{"FromByteArray", "FromRuneArray"})
	instance := "module main\nfn Use(text: string) void {\n    text.\n}\n"
	for _, item := range completeSource("", instance, strings.Index(instance, "text.")+len("text.")) {
		if item.Label == "FromByteArray" || item.Label == "FromRuneArray" {
			t.Fatalf("static constructor in instance completion: %+v", item)
		}
	}
}
