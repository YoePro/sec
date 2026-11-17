package main

import (
	"os"
	"strings"
	"testing"
)

// Attribute completion offers @noPanic before init, free, each property
// accessor and a let that directly initializes a lambda, and hover on such an
// attribute names the real callable body it belongs to with the compiler's
// panic proof instead of the binding, property or impl.
//
// Rules:
//   - rules/foundations/attributes.md — "Callable-body attachment (MD-030)", "LSP completion", "LSP behavior"
//   - rules/corrections/applied/md030-no-panic-attribute-placement-correction-20261017.md — §§ 8(4), 9(5)
func TestCallableBodyAttributeCompletionAndHover(t *testing.T) {
	for _, source := range []string{
		"module main\n\nimpl Counter {\n    @no|\n    init(value: int) {\n    }\n}\n",
		"module main\n\nimpl Counter {\n    @no|\n    free {\n    }\n}\n",
		"module main\n\nimpl Counter {\n    property Value: int {\n        @no|\n        get {\n            return 1\n        }\n    }\n}\n",
		"module main\n\nimpl Counter {\n    property Value: int {\n        @no|\n        try set value CounterError {\n        }\n    }\n}\n",
		"module main\n\nfn Use() void {\n    @no|\n    let identity := fn(value: bool) bool {\n        return value\n    }\n}\n",
		"module main\n\nfn Use() void {\n    @no|\n    let typed: fn(bool) bool := capture(flag) fn(value: bool) bool {\n        return value\n    }\n}\n",
	} {
		offset := strings.Index(source, "|")
		text := source[:offset] + source[offset+1:]
		items, ok := attributeCompletionItems(text, offset)
		if !ok {
			t.Fatalf("%q is not an attribute completion site", source)
		}
		if got := completionLabels(items); got != "noPanic" {
			t.Errorf("%q completions = %s, want noPanic", source, got)
		}
	}
	if items, ok := attributeCompletionItems("module main\n\nfn Use() void {\n    @no\n    let value := 1\n}\n", len("module main\n\nfn Use() void {\n    @no")); ok && strings.Contains(completionLabels(items), "noPanic") {
		t.Errorf("an ordinary local let was offered @noPanic: %s", completionLabels(items))
	}

	data, err := os.ReadFile("../../testdata/sema/no_panic_bodies_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	hoverBefore := func(declaration string) string {
		t.Helper()
		at := strings.Index(text, declaration)
		attribute := strings.LastIndex(text[:at], "@noPanic") + 1
		result, ok := hoverForSource("file:///tmp/sec-lsp-body-attributes/main.sec", text, offsetPosition(text, attribute))
		if !ok {
			t.Fatalf("no hover on @noPanic before %q", declaration)
		}
		return result.Contents.Value
	}
	for declaration, want := range map[string]string{
		"init(value: int)": "Attached to: init `Counter.init`",
		"free {":           "Attached to: free `Counter.free`",
		"get {\n            return self._value\n        }\n\n        @noPanic": "Attached to: property get accessor",
		"try set value CounterError": "Attached to: property try set accessor `Counter.Checked`",
		"let identity :=":            "Attached to: lambda",
	} {
		contents := hoverBefore(declaration)
		for _, expected := range []string{"**@noPanic**", want, "May panic: `no`"} {
			if !strings.Contains(contents, expected) {
				t.Errorf("hover before %q lacks %q:\n%s", declaration, expected, contents)
			}
		}
	}
}
