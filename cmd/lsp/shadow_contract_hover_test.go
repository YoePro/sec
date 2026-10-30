package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShadowContractHover shows a register member's compiler-published
// shadow contract at its declaration and at a call on a hardware binding.
// Rules: rules/tooling/lsp.md — "Register shadow knowledge (MD-113)".
func TestShadowContractHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/shadow_knowledge.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	declaration := strings.Index(source, "fn GetMode") + len("fn ")
	hover, ok := hoverForSource(uri, source, offsetPosition(source, declaration))
	if !ok || !strings.Contains(hover.Contents.Value, "**Shadow contract.** Requires known: `Mode`.") || !strings.Contains(hover.Contents.Value, "not current hardware state") {
		t.Fatal(hover, ok)
	}
	call := strings.Index(source, "control.Refresh()") + len("control.")
	hover, ok = hoverForSource(uri, source, offsetPosition(source, call))
	if !ok || !strings.Contains(hover.Contents.Value, "Ensures known: `Clear`, `Mode`.") {
		t.Fatal(hover, ok)
	}
	overwrite := strings.Index(source, "fn Overwrite") + len("fn ")
	hover, ok = hoverForSource(uri, source, offsetPosition(source, overwrite))
	if !ok || !strings.Contains(hover.Contents.Value, "May make unknown: `Clear`, `Mode`.") {
		t.Fatal(hover, ok)
	}
}
