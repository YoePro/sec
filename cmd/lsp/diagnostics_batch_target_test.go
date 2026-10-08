package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// Open platform files of one module for different targets are analyzed in
// separate Sema runs: the module is assembled for the first document's target,
// so a shared run silently dropped every semantic diagnostic of the other
// platform's file (for example an assignment to an immutable binding).
//
// Rules:
//   - rules/tooling/lsp.md — "Responsiveness model", "Multi-target diagnostics"
//   - rules/platform/platform_model.md — source selection by target
func TestDiagnosticBatchAnalyzesEachPlatformFileForItsTarget(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"shared.sec":             "module device\n\nfn Shared() int {\n    return 1\n}\n",
		"device.freebsd.any.sec": "#target(os: \"freebsd\", arch: \"any\")\nmodule device\n\nfn Platform() int {\n    let mode := 1\n    mode = 2\n    return mode\n}\n",
		"device.linux.any.sec":   "#target(os: \"linux\", arch: \"any\")\nmodule device\n\nfn Platform() int {\n    let mode := 1\n    mode = 2\n    return mode\n}\n",
	}
	snapshots := []lspserver.Snapshot{}
	uris := map[string]string{}
	for name, text := range sources {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		uris[name] = uriFromPath(path)
		snapshots = append(snapshots, lspserver.Snapshot{URI: uris[name], Text: text, Version: 1})
	}
	results := analyzeDiagnosticBatch(snapshots, sourceOverlay{Sources: map[string]string{}})
	for _, name := range []string{"device.freebsd.any.sec", "device.linux.any.sec"} {
		found := false
		for _, item := range results[uris[name]] {
			if strings.Contains(item.Message, "cannot assign to immutable variable mode") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s diagnostics = %+v, want the immutable-assignment error", name, results[uris[name]])
		}
	}
	if len(results[uris["shared.sec"]]) != 0 {
		t.Errorf("shared.sec diagnostics = %+v, want none", results[uris["shared.sec"]])
	}
}
