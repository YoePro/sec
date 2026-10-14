package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	lspserver "sec/internal/lsp/server"
)

func workspaceResolveFixture(t *testing.T, root, filename string) (string, string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../testdata/lsp/workspace_symbols", filename))
	if err != nil {
		t.Fatal(err)
	}
	return writeWorkspaceSource(t, root, filename, string(data)), string(data)
}

// Resolve negotiation and routing must agree with LSP's lazy-range capability,
// retaining exact overloaded/member locations and complete older-client replies.
// Rules: rules/tooling/lsp.md — "Workspace symbols", "Protocol transport and types".
func TestWorkspaceSymbolResolveProtocolNegotiation(t *testing.T) {
	root := t.TempDir()
	workspaceResolveFixture(t, root, "declarations.sec")
	for _, properties := range [][]string{nil, {"containerName"}, {"location.range"}} {
		s := &server{documentSnapshots: lspserver.NewDocuments()}
		initialize := lspTestRequest(t, s, "initialize", map[string]any{
			"rootUri": uriFromPath(root), "capabilities": map[string]any{"workspace": map[string]any{"symbol": map[string]any{"resolveSupport": map[string]any{"properties": properties}}}},
		})
		raw, _ := json.Marshal(initialize.Result)
		if !bytes.Contains(raw, []byte(`"workspaceSymbolProvider":{"resolveProvider":true}`)) {
			t.Fatal(string(raw))
		}
		response := lspTestRequest(t, s, "workspace/symbol", workspaceSymbolParams{Query: "Select"})
		symbols := lspTestDecode[[]workspaceSymbol](t, response)
		if len(symbols) != 3 {
			t.Fatal(symbols)
		}
		lazy := reflect.DeepEqual(properties, []string{"location.range"})
		want := workspaceSymbolsForQuery(s.workspaceSymbols, s.workspaceRoots, "Select", s.sourceOverlay())
		for i, symbol := range symbols {
			if lazy {
				if symbol.Location.Range != nil || symbol.Data == nil {
					t.Fatal(symbol)
				}
				symbol = lspTestDecode[workspaceSymbol](t, lspTestRequest(t, s, "workspaceSymbol/resolve", symbol))
			} else if symbol.Data != nil {
				t.Fatal(symbol)
			}
			if symbol.Location.Range == nil || *symbol.Location.Range != want[i].Location.Range || symbol.Location.URI != want[i].Location.URI || symbol.Name != want[i].Name || symbol.ContainerName != want[i].ContainerName {
				t.Fatalf("resolved %+v, want %+v", symbol, want[i])
			}
			if lazy {
				twice := lspTestDecode[workspaceSymbol](t, lspTestRequest(t, s, "workspaceSymbol/resolve", symbol))
				if !reflect.DeepEqual(twice, symbol) {
					t.Fatal(twice, symbol)
				}
			}
		}
	}
	if !isDeferrableAnalysisRequest("workspaceSymbol/resolve") {
		t.Fatal("resolve blocks priority formatting")
	}
}

// Stale, malformed and out-of-workspace data cannot redirect lazy navigation.
// Rules: rules/tooling/lsp.md — "Workspace symbols", "Document snapshots".
func TestWorkspaceSymbolResolveRejectsStaleAndMalformedItems(t *testing.T) {
	root := t.TempDir()
	path, source := workspaceResolveFixture(t, root, "declarations.sec")
	roots := []string{root}
	index := newWorkspaceSymbolIndex()
	overlay := sourceOverlay{Sources: map[string]string{}}
	symbols := index.unresolvedSymbols(workspaceSymbolsForQuery(index, roots, "Select", overlay))
	original := symbols[0]
	for _, name := range []string{"missing data", "version", "entry", "name", "kind", "container", "uri", "range", "unknown source", "removed workspace"} {
		t.Run(name, func(t *testing.T) {
			symbol := original
			data := *symbol.Data
			symbol.Data = &data
			currentRoots := roots
			switch name {
			case "missing data":
				symbol.Data = nil
			case "version":
				data.Version++
			case "entry":
				data.Entry = 1000
			case "name":
				symbol.Name = "Other"
			case "kind":
				symbol.Kind++
			case "container":
				symbol.ContainerName = "other"
			case "uri":
				symbol.Location.URI = "https://example.invalid/source.sec"
			case "range":
				symbol.Location.Range = &lspRange{}
			case "unknown source":
				symbol.Location.URI = uriFromPath(filepath.Join(root, "unindexed.sec"))
			case "removed workspace":
				currentRoots = nil
			}
			if _, code, _ := index.resolveWorkspaceSymbol(symbol, currentRoots, overlay); code != -32602 {
				t.Fatalf("code = %d", code)
			}
		})
	}
	// A same-size disk edit with the original timestamp still invalidates data.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.ReplaceAll(source, "Select", "Choose")
	if err := os.WriteFile(path, []byte(changed), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if _, code, _ := index.resolveWorkspaceSymbol(original, roots, overlay); code != -32801 {
		t.Fatalf("disk edit code = %d", code)
	}
	fresh := index.unresolvedSymbols(workspaceSymbolsForQuery(index, roots, "Choose", overlay))
	if len(fresh) != 3 {
		t.Fatal(fresh)
	}
	if _, code, _ := index.resolveWorkspaceSymbol(fresh[0], roots, overlay); code != 0 {
		t.Fatalf("fresh code = %d", code)
	}
	overlay.Sources[path] = "\n" + changed
	if _, code, _ := index.resolveWorkspaceSymbol(fresh[0], roots, overlay); code != -32801 {
		t.Fatalf("overlay edit code = %d", code)
	}
	delete(overlay.Sources, path)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, code, _ := index.resolveWorkspaceSymbol(fresh[0], roots, overlay); code != -32801 {
		t.Fatalf("deleted code = %d", code)
	}
	s := &server{workspaceSymbols: index, workspaceRoots: roots}
	for _, input := range []any{nil, 1, map[string]any{"data": "bad"}} {
		response := lspTestRequest(t, s, "workspaceSymbol/resolve", input)
		if response.Error == nil || response.Error.Code != -32602 {
			t.Fatal(response)
		}
	}
}

// Visibility is rechecked at resolve; open-only sources stay accessible without
// a workspace root, and current Unicode coordinates come from document symbols.
// Rules: rules/tooling/lsp.md — "Workspace symbols", "Document snapshots";
// rules/foundations/names_scopes_visibility.md — §12 visibility prefixes.
func TestWorkspaceSymbolResolveVisibilityAndOpenSources(t *testing.T) {
	root := t.TempDir()
	path, source := workspaceResolveFixture(t, root, "declarations.sec")
	sibling, siblingText := workspaceResolveFixture(t, root, "sibling.sec")
	unicode, unicodeText := workspaceResolveFixture(t, root, "unicode.sec")
	index := newWorkspaceSymbolIndex()
	overlay := sourceOverlay{Sources: map[string]string{path: source, sibling: siblingText, unicode: unicodeText}}
	symbols := index.unresolvedSymbols(workspaceSymbolsForQuery(index, nil, "", overlay))
	var internal, private, unicodeSymbol workspaceSymbol
	for _, symbol := range symbols {
		switch symbol.Name {
		case "_Internal":
			internal = symbol
		case "__Private":
			private = symbol
		case "Åter":
			unicodeSymbol = symbol
		}
	}
	resolved, code, _ := index.resolveWorkspaceSymbol(unicodeSymbol, nil, overlay)
	if code != 0 || resolved.Location.Range == nil || resolved.Location.Range.Start.Character != 12 {
		t.Fatal(resolved, code)
	}
	// Repeated resolves and searches share immutable declaration summaries.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				if _, code, _ := index.resolveWorkspaceSymbol(unicodeSymbol, nil, overlay); code != 0 {
					t.Errorf("concurrent code = %d", code)
				}
				_ = index.unresolvedSymbols(workspaceSymbolsForQuery(index, nil, "Åter", overlay))
			}
		}()
	}
	wg.Wait()
	delete(overlay.Sources, path)
	if _, code, _ := index.resolveWorkspaceSymbol(private, []string{root}, overlay); code != -32602 {
		t.Fatalf("private code = %d", code)
	}
	if _, code, _ := index.resolveWorkspaceSymbol(internal, []string{root}, overlay); code != 0 {
		t.Fatalf("sibling module code = %d", code)
	}
	delete(overlay.Sources, sibling)
	delete(overlay.Sources, unicode)
	// A non-Sec open document is not a workspace module context.
	overlay.Sources[filepath.Join(root, "notes.md")] = siblingText
	if _, code, _ := index.resolveWorkspaceSymbol(internal, []string{root}, overlay); code != -32602 {
		t.Fatalf("closed module code = %d", code)
	}
}

// Trusted-core symbols retain their read-only presentation across lazy resolve.
// Rules: rules/tooling/lsp.md — "Workspace symbols".
func TestWorkspaceSymbolResolveCoreContainer(t *testing.T) {
	root := t.TempDir()
	_, source := workspaceResolveFixture(t, root, "declarations.sec")
	writeWorkspaceSource(t, root, "sec/core/declarations.sec", source)
	index := newWorkspaceSymbolIndex()
	overlay := sourceOverlay{Sources: map[string]string{}}
	symbols := index.unresolvedSymbols(workspaceSymbolsForQuery(index, []string{root}, "Select", overlay))
	coreCount := 0
	for _, symbol := range symbols {
		if !strings.HasSuffix(symbol.ContainerName, coreReadOnlyContainerSuffix) {
			continue
		}
		resolved, code, _ := index.resolveWorkspaceSymbol(symbol, []string{root}, overlay)
		if code != 0 || resolved.Location.Range == nil || resolved.ContainerName != symbol.ContainerName || !reflect.DeepEqual(resolved.Data, symbol.Data) {
			t.Fatal(resolved, code)
		}
		coreCount++
	}
	if coreCount != 3 {
		t.Fatalf("core symbols = %d", coreCount)
	}
}
