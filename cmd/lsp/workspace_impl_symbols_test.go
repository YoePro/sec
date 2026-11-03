package main

import (
	"os"
	"strings"
	"testing"
)

// Workspace lookup must retain each extension header, including unopened files
// and generic targets, without merging equal names from distinct modules.
// Rules: rules/declarations/impl.md — §§4–5;
// rules/tooling/lsp.md — "Workspace symbols".
func TestWorkspaceImplExtensionSymbols(t *testing.T) {
	root := t.TempDir()
	sources, paths := map[string]string{}, map[string]string{}
	for _, file := range []struct{ name, fixture string }{
		{"primary", "impl_navigation/primary.sec"},
		{"extension_a", "impl_navigation/extension_a.sec"},
		{"extension_b", "impl_navigation/extension_b.sec"},
		{"other", "workspace_impl_extensions/other.sec"},
	} {
		data, err := os.ReadFile("../../testdata/tooling/" + file.fixture)
		if err != nil {
			t.Fatal(err)
		}
		sources[file.name] = string(data)
		paths[file.name] = writeWorkspaceSource(t, root, file.name+".sec", string(data))
	}
	index := newWorkspaceSymbolIndex()
	symbols := workspaceSymbolsForQuery(index, []string{root}, "impl extends Widget", sourceOverlay{})
	if len(symbols) != 3 {
		t.Fatalf("extension symbols=%v, want three headers", symbols)
	}
	for i, name := range []string{"extension_a", "extension_b", "other"} {
		want := offsetPosition(sources[name], strings.Index(sources[name], "impl extends Widget")+len("impl extends "))
		if symbols[i].Name != "impl extends Widget" || symbols[i].Location.URI != uriFromPath(paths[name]) || symbols[i].Location.Range.Start != want {
			t.Fatalf("header=%v, want %s at %v", symbols[i], name, want)
		}
	}
	generic := workspaceSymbolsForQuery(index, []string{root}, "impl extends Box", sourceOverlay{})
	if len(generic) != 1 || generic[0].Location.URI != uriFromPath(paths["extension_a"]) {
		t.Fatalf("generic extension=%v", generic)
	}
	// Use the existing lazy protocol and declaration snapshot for every header.
	for _, item := range index.unresolvedSymbols(symbols) {
		if item.Data == nil || item.Location.Range != nil {
			t.Fatalf("header not lazily indexed: %v", item)
		}
		resolved, code, message := index.resolveWorkspaceSymbol(item, []string{root}, sourceOverlay{})
		if code != 0 || resolved.Location.Range == nil {
			t.Fatalf("resolve=%v %d %s", resolved, code, message)
		}
	}
}

// Header visibility comes from the target, including a private qualified
// component. Resolve must recheck the same access after documents close.
// Rules: rules/tooling/lsp.md — "Workspace symbols";
// rules/foundations/names_scopes_visibility.md — §12.
func TestWorkspaceImplExtensionVisibility(t *testing.T) {
	root := t.TempDir()
	data, err := os.ReadFile("../../testdata/tooling/workspace_impl_extensions/hidden.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	path := writeWorkspaceSource(t, root, "hidden.sec", source)
	primary, err := os.ReadFile("../../testdata/tooling/impl_navigation/primary.sec")
	if err != nil {
		t.Fatal(err)
	}
	sibling := writeWorkspaceSource(t, root, "primary.sec", string(primary))
	index := newWorkspaceSymbolIndex()
	if got := workspaceSymbolsForQuery(index, []string{root}, "impl extends", sourceOverlay{}); len(got) != 0 {
		t.Fatalf("hidden target headers exposed: %v", got)
	}
	overlay := sourceOverlay{Sources: map[string]string{sibling: string(primary)}}
	internal := workspaceSymbolsForQuery(index, []string{root}, "impl extends", overlay)
	if len(internal) != 1 || internal[0].Name != "impl extends _Internal" {
		t.Fatalf("module visibility=%v", internal)
	}
	lazyInternal := index.unresolvedSymbols(internal)[0]
	overlay.Sources[path] = source
	all := workspaceSymbolsForQuery(index, []string{root}, "impl extends", overlay)
	if len(all) != 3 {
		t.Fatalf("open-file visibility=%v", all)
	}
	lazy := index.unresolvedSymbols(all)
	delete(overlay.Sources, path)
	for _, item := range lazy {
		_, code, message := index.resolveWorkspaceSymbol(item, []string{root}, overlay)
		if item.Name == "impl extends _Internal" {
			if code != 0 {
				t.Fatalf("internal no longer visible to sibling: %d %s", code, message)
			}
		} else if code != -32602 {
			t.Fatalf("private header %s resolved after close: %d %s", item.Name, code, message)
		}
	}
	delete(overlay.Sources, sibling)
	if _, code, _ := index.resolveWorkspaceSymbol(lazyInternal, []string{root}, overlay); code != -32602 {
		t.Fatalf("closed module resolve=%d", code)
	}
}

// Equal header names retain distinct declaration indices; edits/removal must
// invalidate their snapshots and the cached inventory follows open overlays.
// Rules: rules/tooling/lsp.md — "Workspace symbols", "Document snapshots".
func TestWorkspaceImplExtensionSnapshots(t *testing.T) {
	root := t.TempDir()
	primary, err := os.ReadFile("../../testdata/tooling/impl_navigation/primary.sec")
	if err != nil {
		t.Fatal(err)
	}
	writeWorkspaceSource(t, root, "primary.sec", string(primary))
	data, err := os.ReadFile("../../testdata/tooling/workspace_impl_extensions/multiple.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	path := writeWorkspaceSource(t, root, "multiple.sec", source)
	index := newWorkspaceSymbolIndex()
	symbols := workspaceSymbolsForQuery(index, []string{root}, "impl extends Widget", sourceOverlay{})
	if len(symbols) != 2 {
		t.Fatalf("same-file extensions=%v", symbols)
	}
	lazy := index.unresolvedSymbols(symbols)
	if lazy[0].Data == nil || lazy[1].Data == nil || lazy[0].Data.Entry == lazy[1].Data.Entry {
		t.Fatalf("header identities merged: %v", lazy)
	}
	for i, item := range lazy {
		resolved, code, message := index.resolveWorkspaceSymbol(item, []string{root}, sourceOverlay{})
		if code != 0 || resolved.Location.Range == nil || *resolved.Location.Range != symbols[i].Location.Range {
			t.Fatalf("resolve[%d]=%v %d %s", i, resolved, code, message)
		}
	}
	edited := "// unsaved change\n" + strings.ReplaceAll(source, "impl extends Widget", "impl extends Box[T]")
	overlay := sourceOverlay{Sources: map[string]string{path: edited}}
	if got := workspaceSymbolsForQuery(index, []string{root}, "impl extends Widget", overlay); len(got) != 0 {
		t.Fatalf("stale target in index: %v", got)
	}
	if got := workspaceSymbolsForQuery(index, []string{root}, "impl extends Box", overlay); len(got) != 2 {
		t.Fatalf("edited target missing: %v", got)
	}
	if _, code, _ := index.resolveWorkspaceSymbol(lazy[0], []string{root}, overlay); code != -32801 {
		t.Fatalf("stale header resolve=%d", code)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := workspaceSymbolsForQuery(index, nil, "impl extends Box", overlay); len(got) != 2 {
		t.Fatalf("open-only headers lost: %v", got)
	}
	delete(overlay.Sources, path)
	if got := workspaceSymbolsForQuery(index, []string{root}, "impl extends", overlay); len(got) != 0 {
		t.Fatalf("deleted headers retained: %v", got)
	}
}
