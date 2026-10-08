package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeWorkspaceSource(t *testing.T, dir string, name string, text string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return normalizedSourcePath(path)
}

func workspaceSymbolNames(symbols []symbolInformation) []string {
	names := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		names = append(names, symbol.Name+"@"+symbol.ContainerName)
	}
	return names
}

func TestWorkspaceSymbolsFindOpenAndUnopenedFiles(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceSource(t, root, "app/main.sec", "module app\n\nfn RunServer() void {\n}\n")
	geometry := writeWorkspaceSource(t, root, "geometry/shape.sec", `module geometry

type Point struct {
	x: int,
	y: int,
}

type Color enum {
	Red,
	Green,
}

type Shape union {
	Circle(int),
	Square(int),
}

impl Point {
	fn Length() int {
		return 0
	}
}
`)
	writeWorkspaceSource(t, root, "testdata/fixture.sec", "module fixture\n\nfn RunFixture() void {\n}\n")
	open := writeWorkspaceSource(t, root, "app/editing.sec", "module app\n")
	overlay := sourceOverlay{Sources: map[string]string{open: "module app\n\nfn RunEdited() void {\n}\n"}}

	index := newWorkspaceSymbolIndex()
	symbols := workspaceSymbolsForQuery(index, []string{normalizedSourcePath(root)}, "run", overlay)
	got := strings.Join(workspaceSymbolNames(symbols), ",")
	if got != "RunEdited@app,RunServer@app" {
		t.Fatalf("run symbols = %s", got)
	}

	symbols = workspaceSymbolsForQuery(index, []string{normalizedSourcePath(root)}, "", overlay)
	names := map[string]symbolInformation{}
	for _, symbol := range symbols {
		names[symbol.Name+"@"+symbol.ContainerName] = symbol
	}
	for _, want := range []string{"Point@geometry", "x@Point", "Length@Point", "Red@Color", "Circle@Shape", "Shape@geometry"} {
		if _, ok := names[want]; !ok {
			t.Fatalf("missing %s in %v", want, workspaceSymbolNames(symbols))
		}
	}
	length := names["Length@Point"]
	if length.Location.URI != uriFromPath(geometry) || length.Location.Range.Start.Line != 18 || length.Kind != 6 {
		t.Fatalf("Length location = %#v", length)
	}

	// The cached summary follows a changed open snapshot.
	overlay.Sources[open] = "module app\n\nfn RunRenamed() void {\n}\n"
	got = strings.Join(workspaceSymbolNames(workspaceSymbolsForQuery(index, []string{normalizedSourcePath(root)}, "run", overlay)), ",")
	if got != "RunRenamed@app,RunServer@app" {
		t.Fatalf("run symbols after change = %s", got)
	}
}

func TestWorkspaceSymbolsMatchCaseInsensitivelyInDeterministicOrder(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceSource(t, root, "a.sec", "module a\n\nfn ParseHeader() void {\n}\n\nfn Parse() void {\n}\n\nfn PeerAddress() void {\n}\n\nfn ReparseAll() void {\n}\n")
	symbols := workspaceSymbolsForQuery(newWorkspaceSymbolIndex(), []string{normalizedSourcePath(root)}, "PARSE", sourceOverlay{Sources: map[string]string{}})
	got := strings.Join(workspaceSymbolNames(symbols), ",")
	if got != "Parse@a,ParseHeader@a,ReparseAll@a" {
		t.Fatalf("PARSE symbols = %s", got)
	}
	symbols = workspaceSymbolsForQuery(newWorkspaceSymbolIndex(), []string{normalizedSourcePath(root)}, "pa", sourceOverlay{Sources: map[string]string{}})
	got = strings.Join(workspaceSymbolNames(symbols), ",")
	if got != "Parse@a,ParseHeader@a,ReparseAll@a,PeerAddress@a" {
		t.Fatalf("pa symbols = %s", got)
	}
}

func TestWorkspaceSymbolsHidePrivateDeclarationsOfOtherModules(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceSource(t, root, "store/store.sec", "module store\n\nfn _InternalFlush() void {\n}\n\nfn __PrivateHelper() void {\n}\n\nfn Flush() void {\n}\n")
	sibling := writeWorkspaceSource(t, root, "store/other.sec", "module store\n")
	roots := []string{normalizedSourcePath(root)}

	got := strings.Join(workspaceSymbolNames(workspaceSymbolsForQuery(newWorkspaceSymbolIndex(), roots, "", sourceOverlay{Sources: map[string]string{}})), ",")
	if got != "Flush@store" {
		t.Fatalf("symbols without open store document = %s", got)
	}
	// An open document of the same module makes module-internal names
	// visible, but a private name stays owned by its unopened file.
	overlay := sourceOverlay{Sources: map[string]string{sibling: "module store\n"}}
	got = strings.Join(workspaceSymbolNames(workspaceSymbolsForQuery(newWorkspaceSymbolIndex(), roots, "", overlay)), ",")
	if got != "Flush@store,_InternalFlush@store" {
		t.Fatalf("symbols with open store document = %s", got)
	}
}

func TestWorkspaceSymbolsMarkCoreDeclarationsReadOnly(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceSource(t, root, "sec/core/text.sec", "module core\n\nfn Trim() void {\n}\n")
	writeWorkspaceSource(t, root, "app/text.sec", "module app\n\nfn Trim() void {\n}\n")
	symbols := workspaceSymbolsForQuery(newWorkspaceSymbolIndex(), []string{normalizedSourcePath(root)}, "trim", sourceOverlay{Sources: map[string]string{}})
	got := strings.Join(workspaceSymbolNames(symbols), ",")
	if got != "Trim@app,Trim@core"+coreReadOnlyContainerSuffix {
		t.Fatalf("trim symbols = %s", got)
	}
}

func TestWorkspaceRootsFollowInitializeAndFolderChanges(t *testing.T) {
	roots := workspaceRootsFromInitialize(initializeParams{RootURI: "file:///work/a"})
	if len(roots) != 1 || roots[0] != normalizedSourcePath("/work/a") {
		t.Fatalf("roots = %v", roots)
	}
	var change didChangeWorkspaceFoldersParams
	change.Event.Added = []workspaceFolder{{URI: "file:///work/b"}}
	change.Event.Removed = []workspaceFolder{{URI: "file:///work/a"}}
	roots = changedWorkspaceRoots(roots, change)
	if len(roots) != 1 || roots[0] != normalizedSourcePath("/work/b") {
		t.Fatalf("roots after change = %v", roots)
	}
}
