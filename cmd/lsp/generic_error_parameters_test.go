package main

import (
	"strings"
	"testing"
)

// Hover distinguishes a constrained error parameter (MD-120 § 10(7); rules/tooling/lsp.md "Hover").
func TestGenericErrorParameterHover(t *testing.T) {
	source := "module main\nfn Wrap[E: error](cause: E) Result[void, E] {\n    return Err(<-cause)\n}\n"
	hover, ok := hoverForSource("file:///tmp/sec-lsp-generic-error/main.sec", source, offsetPosition(source, strings.Index(source, "E: error")))
	if !ok || !strings.Contains(hover.Contents.Value, "generic E: error") {
		t.Fatalf("hover = %+v, ok = %v", hover, ok)
	}
	for _, item := range analyze("file:///tmp/sec-lsp-generic-error/main.sec", source) {
		if item.Severity == 1 {
			t.Fatalf("valid error parameter: %+v", item)
		}
	}
}
