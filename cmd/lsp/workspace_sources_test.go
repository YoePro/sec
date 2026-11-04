package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// Project source discovery and lazy resolve share the same manifest boundary;
// opening one module does not hide the rest of its project or publish outputs.
// Rules: rules/tooling/lsp.md — Workspace symbols, Project loading;
// rules/projects/projects.md — §§13–15,36,49.
func TestWorkspaceSymbolsUseProjectSourceRoots(t *testing.T) {
	root := t.TempDir()
	manifest, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := writeWorkspaceSource(t, root, ".sec/sec.toml", string(manifest))
	source, err := os.ReadFile("../../testdata/lsp/workspace_symbols/sibling.sec")
	if err != nil {
		t.Fatal(err)
	}
	valid := []string{"main.sec", "cmd/app/main.sec", "services/store/store.sec", "internal/store/store.sec", "services/bin/store.sec"}
	invalid := []string{"bin/output.sec", "bin/cache/old.sec", ".sec/cache.sec", "cmd/invalid.sec", "internal/invalid.sec", "services/internal/invalid.sec", "testdata/example.sec", "node_modules/example.sec", ".hidden/example.sec"}
	for _, name := range append(append([]string{}, valid...), invalid...) {
		writeWorkspaceSource(t, root, name, string(source))
	}
	roots := []string{filepath.Join(root, "services", "store"), root}
	overlay := sourceOverlay{Sources: map[string]string{}}
	want := []string{}
	for _, name := range valid {
		want = append(want, normalizedSourcePath(filepath.Join(root, name)))
	}
	sort.Strings(want)
	if got := workspaceSourcePaths(roots, overlay); !reflect.DeepEqual(got, want) {
		t.Fatalf("project sources = %v, want %v", got, want)
	}
	index := newWorkspaceSymbolIndex()
	symbols := index.unresolvedSymbols(workspaceSymbolsForQuery(index, roots[:1], "", overlay))
	if len(symbols) == 0 {
		t.Fatal("empty project symbol query")
	}
	foundSibling := false
	for _, symbol := range symbols {
		if symbol.Location.URI == uriFromPath(filepath.Join(root, "main.sec")) {
			foundSibling = true
		}
		if _, code, reason := index.resolveWorkspaceSymbol(symbol, roots[:1], overlay); code != 0 {
			t.Fatal("project query/resolve disagree", code, reason)
		}
	}
	if !foundSibling {
		t.Fatal("opening a project module hid its siblings")
	}
	server := &server{workspaceSymbols: index, workspaceRoots: roots[:1]}
	response := lspTestRequest(t, server, "workspace/symbol", workspaceSymbolParams{})
	protocolSymbols := lspTestDecode[[]symbolInformation](t, response)
	if len(protocolSymbols) != len(symbols) {
		t.Fatalf("protocol symbols = %d, query symbols = %d", len(protocolSymbols), len(symbols))
	}
	// Reuse the symbol cache while the project becomes invalid. Neither query
	// nor resolve may publish old symbols through loose-folder fallback.
	if err := os.WriteFile(manifestPath, []byte("[project]\nname = 123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := workspaceSymbolsForQuery(index, roots, "", overlay); len(got) != 0 {
		t.Fatal("invalid manifest leaked cached symbols", got)
	}
	if _, code, _ := index.resolveWorkspaceSymbol(symbols[0], roots, overlay); code != -32602 {
		t.Fatal("invalid manifest allowed resolve", code)
	}
	if err := os.WriteFile(manifestPath, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if got := workspaceSourcePaths(roots, overlay); !reflect.DeepEqual(got, want) {
		t.Fatal("manifest repair retained stale scope", got)
	}
	if err := os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}
	if got := workspaceSourcePaths(roots[:1], overlay); !reflect.DeepEqual(got, []string{normalizedSourcePath(filepath.Join(root, "services/store/store.sec"))}) {
		t.Fatal("manifest removal retained expanded project scope", got)
	}
	// Explicit open documents remain discoverable outside project scope,
	// including invalid source locations, so users can navigate and fix them.
	open := normalizedSourcePath(filepath.Join(root, "bin/output.sec"))
	overlay.Sources[open] = string(source)
	if !workspaceSymbolSourceInScope(open, nil, overlay) || len(workspaceSourcePaths(nil, overlay)) != 1 {
		t.Fatal("explicit open-document inclusion changed")
	}
}

// Independent projects can share an editor folder; an inner manifest within
// one project cannot authorize a nested project's declarations.
// Rules: rules/projects/projects.md — §§4(1–3),36; rules/tooling/lsp.md — Workspace symbols.
func TestWorkspaceSymbolsProjectBoundaries(t *testing.T) {
	root := t.TempDir()
	manifest, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/lsp/workspace_symbols/sibling.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first", "second"} {
		writeWorkspaceSource(t, root, name+"/.sec/sec.toml", string(manifest))
		writeWorkspaceSource(t, root, name+"/module/source.sec", string(source))
		writeWorkspaceSource(t, root, name+"/bin/output.sec", string(source))
	}
	writeWorkspaceSource(t, root, "first/nested/.sec/sec.toml", string(manifest))
	nested := writeWorkspaceSource(t, root, "first/nested/source.sec", string(source))
	overlay := sourceOverlay{Sources: map[string]string{}}
	want := []string{filepath.Join(root, "first/module/source.sec"), filepath.Join(root, "second/module/source.sec")}
	if got := workspaceSourcePaths([]string{root}, overlay); !reflect.DeepEqual(got, want) {
		t.Fatalf("umbrella sources = %v, want %v", got, want)
	}
	if workspaceSymbolSourceInScope(nested, []string{filepath.Join(root, "first")}, overlay) {
		t.Fatal("nested manifest bypassed enclosing project")
	}
}
