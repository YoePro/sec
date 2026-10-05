package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// While editing a command Target's source, the LSP reports the entry
// contract the compiler enforces at build time; a library Target's source
// may declare any main.
//
// Rules:
//   - rules/compiler/initialization.md — § 21 "Command entry", § 26 "Library targets", § 42 "Diagnostics"
func TestLSPReportsProgramEntryContract(t *testing.T) {
	root := t.TempDir()
	write := func(relative string, text string) string {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write(".sec/sec.toml", "[project]\nname = \"tool\"\n\n[target.tool]\nkind = \"command\"\nsource = \"cmd/tool\"\n\n[target.core]\nkind = \"library\"\nsource = \"internal/core\"\n")
	command := write("cmd/tool/main.sec", "module main\n\nfn main() void {\n}\n")
	library := write("internal/core/core.sec", "module core\n\nfn main(value: int) int {\n    return value\n}\n")
	snapshots := []lspserver.Snapshot{}
	for _, path := range []string{command, library} {
		data, _ := os.ReadFile(path)
		snapshots = append(snapshots, lspserver.Snapshot{URI: uriFromPath(path), Text: string(data), Version: 1})
	}
	results := analyzeDiagnosticBatch(snapshots, sourceOverlay{})
	found := false
	for _, item := range results[uriFromPath(command)] {
		if item.Code == "S1110" && strings.Contains(item.Message, "command entry must be `fn main() int`; this main returns void") && item.Range.Start.Line == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("command diagnostics = %+v, want S1110 on main", results[uriFromPath(command)])
	}
	if len(results[uriFromPath(library)]) != 0 {
		t.Errorf("library diagnostics = %+v, want none", results[uriFromPath(library)])
	}
}
