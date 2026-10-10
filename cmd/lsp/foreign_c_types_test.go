package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Outside a project the LSP analyzes for the host target like the compiler
// CLI, so c:: fundamental types resolve through the host C ABI model instead
// of failing only in the editor (ffi.md §5, governance frontend.ffi-v2).
//
// Rules:
//   - rules/tooling/lsp.md — "Target-aware analysis"
//   - rules/platform/ffi.md — §5 "Fundamental C ABI scalar family"
func TestHoverResolvesCFundamentalTypesOutsideProjects(t *testing.T) {
	source := "module main\n\nextern \"C\" fn GetVersion() c::int\n\nfn Main() int {\n    let version := GetVersion()\n    discard version\n    return 0\n}\n"
	path := filepath.Join(t.TempDir(), "main.sec")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	offset := strings.Index(source, "version :=")
	for _, uri := range []string{"file://" + path, ""} {
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, "version: c::int") {
			t.Fatalf("hover %q = %v %q, want version: c::int", uri, ok, hover.Contents.Value)
		}
	}
}
