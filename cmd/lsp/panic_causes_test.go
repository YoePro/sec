package main

import (
	"os"
	"strings"
	"testing"
)

// TestPanicHoverShowsAllCauses consumes the shared multi-cause compiler summary.
// Rules: rules/errors/panic.md — §§25(3),28(6)–(7); rules/tooling/lsp.md — "Hover".
func TestPanicHoverShowsAllCauses(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/panic_causes_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := "file:///tmp/panic-causes/main.sec"
	hover, ok := hoverForSource(uri, source, offsetPosition(source, strings.Index(source, "Root(value")))
	if !ok {
		t.Fatal("missing Root hover")
	}
	for _, want := range []string{"May panic: `yes`", "Panic cause in `Check`", "Panic cause in `Divide`", "Panic cause in `Stop`", "AssertionFailed", "DivisionByZero", "ArithmeticOverflow", `message "value must hold"`, `message "stop here"`, "Called from `Root` at /tmp/panic-causes/main.sec:", "Called from `Middle` at /tmp/panic-causes/main.sec:"} {
		if !strings.Contains(hover.Contents.Value, want) {
			t.Fatalf("missing %q:\n%s", want, hover.Contents.Value)
		}
	}
}
