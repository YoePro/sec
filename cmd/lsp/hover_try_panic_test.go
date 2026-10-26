package main

import (
	"strings"
	"testing"
)

// Try hover shows the panic paths the try leaves in its protected expression
// (a call whose target may panic) and shows none where the try converts every
// failure.
//
// Rules:
//   - rules/tooling/lsp.md — try hover "remaining panic effect"
//   - rules/errors/errorhandling.md — § 31 "try does not catch panic"
func TestTryHoverShowsRemainingPanicEffect(t *testing.T) {
	source := `module main

enum Failure error {
    Missing,
}

fn Risky(value: int32) Result[int32, Failure] {
    assert value != 7
    return Ok(value)
}

fn Use(a: int32, b: int32) Result[int32, error] {
    let risky := try Risky(a)
    let total := try a + b
    return Ok(risky + total)
}
`
	hover, ok := hoverForSource("", source, offsetPosition(source, strings.Index(source, "try Risky")+1))
	if !ok || !strings.Contains(hover.Contents.Value, "Remaining panic effect: `may panic` (try does not catch panic)") ||
		!strings.Contains(hover.Contents.Value, "call to `Risky` at line 13 may panic: `AssertionFailed`") {
		t.Fatalf("try Risky hover = %+v, %v", hover, ok)
	}
	hover, ok = hoverForSource("", source, offsetPosition(source, strings.Index(source, "try a + b")+1))
	if !ok || strings.Contains(hover.Contents.Value, "Remaining panic effect") {
		t.Fatalf("converted arithmetic leaves no panic path: %+v, %v", hover, ok)
	}
}
