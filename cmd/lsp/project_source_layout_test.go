package main

import (
	"os"
	"path/filepath"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// Rules: rules/projects/projects.md — §§14–15; rules/tooling/lsp.md — snapshots and shared diagnostics.
func TestLSPProjectSourceLayoutOverlay(t *testing.T) {
	root := t.TempDir()
	manifest, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/init/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	// The governing manifest exists only in the unsaved overlay.
	overlay := sourceOverlay{Sources: lspserver.SourceOverlay{normalizedSourcePath(filepath.Join(root, ".sec", "sec.toml")): string(manifest)}}
	for _, directory := range []string{"cmd", "bin/store", ".sec/state", "services/internal"} {
		path := filepath.Join(root, directory, "main.sec")
		items := analyze(uriFromPath(path), string(source), overlay)
		found := false
		for _, item := range items {
			if item.Code == "S1149" {
				found = true
				if item.Range.Start.Line != 0 || item.Range.Start.Character != 7 || item.Range.End.Character != 11 {
					t.Fatal(item)
				}
			}
		}
		if !found {
			t.Fatal(directory, items)
		}
	}
	importSource, err := os.ReadFile("../../testdata/projects/source_layout/internal_import.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "client", "client.sec")
	items := analyze(uriFromPath(path), string(importSource), overlay)
	for _, item := range items {
		if item.Code == "S1149" {
			return
		}
	}
	t.Fatal(items)
}
