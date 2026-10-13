package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// Rules: rules/projects/projects.md — §14; rules/tooling/lsp.md — shared compiler workspace.
func TestLSPProjectRootModuleOverlay(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	disk := strings.Replace(string(fixture), `module = "suite"`, `module = "codec"`, 1)
	if err := os.WriteFile(manifest, []byte(disk), 0644); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/root_module/library.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "library.sec")
	if err := os.WriteFile(path, source, 0644); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(path)
	results := analyze(uri, string(source), sourceOverlay{Sources: map[string]string{}})
	for _, diagnostic := range results {
		if diagnostic.Code == "S1148" {
			t.Fatal(results)
		}
	}
	overlay := sourceOverlay{Sources: lspserver.SourceOverlay{normalizedSourcePath(manifest): strings.Replace(disk, `module = "codec"`, `module = "different"`, 1)}}
	results = analyze(uri, string(source), overlay)
	found := false
	for _, diagnostic := range results {
		if diagnostic.Code == "S1148" {
			found = true
			if diagnostic.Range.Start.Line != 0 || diagnostic.Range.Start.Character != 7 || diagnostic.Range.End.Character != 12 {
				t.Fatal(diagnostic)
			}
		}
	}
	if !found {
		t.Fatal(results)
	}
	// Changing a root override in an unsaved manifest is visible immediately.
	results = analyze(uri, string(source), sourceOverlay{Sources: map[string]string{normalizedSourcePath(manifest): disk}})
	for _, diagnostic := range results {
		if diagnostic.Code == "S1148" {
			t.Fatal(results)
		}
	}
}
