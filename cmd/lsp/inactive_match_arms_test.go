package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInactiveMatchArmLSP reports the same A2008 identity, Information
// severity and pattern location as the CLI, and hover on the inactive arm
// shows the compiler's reachable-state proof.
// Rules: rules/tooling/lsp.md — "Error patterns, inactive arms and `Some`";
// rules/tooling/diagnostics.md — § 24(4e).
func TestInactiveMatchArmLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/inactive_match_arm.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	items := analyze(uri, source)
	if len(items) != 1 {
		t.Fatal(items)
	}
	d := items[0]
	if d.Code != "A2008" || d.Severity != 3 || d.Range.Start.Line != 11 || d.Range.Start.Character != 8 || !strings.Contains(d.Message, "this match arm cannot be selected") {
		t.Fatal(d)
	}
	position := offsetPosition(source, strings.Index(source, "Line(length)"))
	hover, ok := hoverForSource(uri, source, position)
	if !ok || !strings.Contains(hover.Contents.Value, "**Inactive arm.** `figure` is proven to contain `Figure.Dot` here (from its construction at line 9)") {
		t.Fatal(hover, ok)
	}
	position = offsetPosition(source, strings.Index(source, "Dot =>"))
	if hover, ok := hoverForSource(uri, source, position); ok && strings.Contains(hover.Contents.Value, "Inactive arm") {
		t.Fatal(hover)
	}
}
