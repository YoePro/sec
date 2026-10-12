package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"sec/internal/lsp/protocol"
	lspserver "sec/internal/lsp/server"
)

// panicRefreshFixture keeps Sec sources in testdata.
// Rules: rules/errors/panic.md — §§17,21,28(8).
func panicRefreshFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/lsp/panic_refresh/" + name + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestPanicDependencyInvalidation exercises the protocol's incremental source
// closure against cold Sema diagnostics and fresh hover/call-hierarchy facts.
// No separate LSP panic analysis or persistent summary cache is introduced.
// Rules: rules/errors/panic.md — §28(6)–(8);
// rules/compiler/incremental_compilation.md — §§13,57–58;
// rules/tooling/lsp.md — "Snapshots", "Incremental analysis".
func TestPanicDependencyInvalidation(t *testing.T) {
	root := t.TempDir()
	mainFile := filepath.Join(root, "app", "main.sec")
	helperFile := filepath.Join(root, "helper", "helper.sec")
	for _, dir := range []string{".sec", "app", "helper"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte("[project]\nname = \"panic-refresh\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mainSource := panicRefreshFixture(t, "main")
	free := panicRefreshFixture(t, "helper_free")
	for file, source := range map[string]string{mainFile: mainSource, helperFile: free} {
		if err := os.WriteFile(file, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	mainURI, helperURI := uriFromPath(mainFile), uriFromPath(helperFile)
	otherURI := uriFromPath(filepath.Join(t.TempDir(), "other.sec"))
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	s.documentSnapshots.Open(mainURI, 1, mainSource)
	s.documentSnapshots.Open(helperURI, 1, free)
	s.documentSnapshots.Open(otherURI, 1, free)
	for _, uri := range []string{mainURI, helperURI, otherURI} {
		if err := s.publishModuleDiagnostics(uri); err != nil {
			t.Fatal(err)
		}
	}
	dir := normalizedSourcePath(filepath.Dir(mainFile))
	otherDir := normalizedSourcePath(filepath.Dir(pathFromURI(otherURI)))
	unchanged := s.diagnosticGeneration[otherDir]
	if !s.diagnosticDependencies[dir][normalizedSourcePath(helperFile)] {
		t.Fatal("import not in compiler source closure")
	}
	s.crossTarget = newCrossTargetStore()
	s.crossTarget.results[otherURI] = crossTargetResult{textHash: documentTextHash(free)}
	event := func(method string, params any) {
		t.Helper()
		data, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.handle(rpcMessage{Method: method, Params: data}); err != nil {
			t.Fatal(err)
		}
	}
	change := func(uri, text string, version int) {
		t.Helper()
		event("textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": version}, "contentChanges": []map[string]string{{"text": text}}})
	}
	verify := func(old uint64, mayPanic bool) {
		t.Helper()
		if s.diagnosticGeneration[dir] <= old {
			t.Fatal("dependent panic summary not invalidated")
		}
		out.Reset()
		if err := s.publishDiagnosticBatch(dir, old); err != nil {
			t.Fatal(err)
		}
		if out.Len() != 0 {
			t.Fatal("superseded panic diagnostics published")
		}
		if err := s.publishDiagnosticBatch(dir, s.diagnosticGeneration[dir]); err != nil {
			t.Fatal(err)
		}
		message, err := protocol.ReadMessage(bufio.NewReader(bytes.NewReader(out.Bytes())))
		if err != nil {
			t.Fatal(err)
		}
		var published struct {
			URI         string       `json:"uri"`
			Diagnostics []diagnostic `json:"diagnostics"`
		}
		if err := json.Unmarshal(message.Params, &published); err != nil {
			t.Fatal(err)
		}
		current, _ := s.documentSnapshots.Snapshot(mainURI)
		cold := analyze(mainURI, current.Text, s.sourceOverlay())
		if published.URI != mainURI || !reflect.DeepEqual(published.Diagnostics, cold) {
			t.Fatalf("incremental diagnostics differ from cold analysis: %+v / %+v", published, cold)
		}
		wantCount := 0
		if mayPanic {
			wantCount = 1
		}
		if strings.Contains(out.String(), "S1092") != mayPanic || len(cold) != wantCount {
			t.Fatalf("unexpected panic diagnostics: %s", out.String())
		}
		pos := offsetPosition(current.Text, strings.Index(current.Text, "Root() void"))
		hover, ok := hoverForSource(mainURI, current.Text, pos, s.sourceOverlay())
		want := "May panic: `no`"
		if mayPanic {
			want = "May panic: `yes`"
		}
		if !ok || !strings.Contains(hover.Contents.Value, want) {
			t.Fatalf("stale panic hover: %+v", hover)
		}
		items := callHierarchyItemsForSource(mainURI, current.Text, pos, s.sourceOverlay())
		if len(items) != 1 || strings.Contains(items[0].Detail, "may panic") != mayPanic {
			t.Fatalf("stale call hierarchy: %+v", items)
		}
	}
	for index, name := range []string{"helper_panic", "helper_free", "helper_untrusted", "helper_trusted", "helper_unproven", "helper_proven", "helper_generic_panic", "helper_generic_free", "helper_cleanup_panic", "helper_cleanup_free"} {
		old := s.diagnosticGeneration[dir]
		s.crossTarget.results[mainURI] = crossTargetResult{textHash: documentTextHash(mainSource)}
		crossGeneration := s.crossTarget.generation[mainURI]
		change(helperURI, panicRefreshFixture(t, name), index+2)
		if _, ok := s.crossTargetResultFor(mainURI, mainSource); ok || s.crossTarget.generation[mainURI] <= crossGeneration {
			t.Fatal("imported stale cross-target panic facts retained")
		}
		if s.diagnosticGeneration[otherDir] != unchanged {
			t.Fatal("unrelated module invalidated")
		}
		if _, ok := s.crossTargetResultFor(otherURI, free); !ok {
			t.Fatal("unrelated cross-target facts invalidated")
		}
		verify(old, name == "helper_panic" || name == "helper_untrusted" || name == "helper_unproven" || name == "helper_generic_panic" || name == "helper_cleanup_panic")
	}
	// Closing an unsaved panic body returns to the panic-free disk source.
	change(helperURI, panicRefreshFixture(t, "helper_panic"), 20)
	old := s.diagnosticGeneration[dir]
	event("textDocument/didClose", map[string]any{"textDocument": map[string]string{"uri": helperURI}})
	verify(old, false)
	// A new, unsaved sibling changes the imported declaration set. It must
	// invalidate dependents even though it was absent from their old file list.
	siblingURI := uriFromPath(filepath.Join(root, "helper", "sibling.sec"))
	old = s.diagnosticGeneration[dir]
	event("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": siblingURI, "version": 1, "text": panicRefreshFixture(t, "sibling")}})
	verify(old, false) // An uncalled panic source must not infect Root.
	if !s.diagnosticDependencies[dir][normalizedSourcePath(pathFromURI(siblingURI))] {
		t.Fatal("new sibling missing from refreshed closure")
	}
	old = s.diagnosticGeneration[dir]
	change(mainURI, panicRefreshFixture(t, "main_sibling"), 2)
	verify(old, true)
	// Dropping the import must remove its dependencies and its panic effects.
	old = s.diagnosticGeneration[dir]
	change(mainURI, panicRefreshFixture(t, "main_local"), 3)
	verify(old, false)
	old = s.diagnosticGeneration[dir]
	change(siblingURI, panicRefreshFixture(t, "sibling_free"), 2)
	if s.diagnosticGeneration[dir] != old {
		t.Fatal("removed import still invalidates Root")
	}
	// Restore the import; a closed dependency changed on disk is refreshed by
	// the watched-file protocol, including assertion proof and cause locations.
	old = s.diagnosticGeneration[dir]
	change(mainURI, mainSource, 4)
	verify(old, false)
	if err := os.WriteFile(helperFile, []byte(panicRefreshFixture(t, "helper_unproven")), 0644); err != nil {
		t.Fatal(err)
	}
	old = s.diagnosticGeneration[dir]
	event("workspace/didChangeWatchedFiles", map[string]any{"changes": []map[string]any{{"uri": helperURI, "type": 2}}})
	verify(old, true)
}

// TestPanicPlanInvalidation reloads target-selected imported implementations
// after manifest changes, rejecting old diagnostic and cross-plan generations.
// Rules: rules/errors/panic.md — §28(8); rules/tooling/lsp.md — "Snapshots";
// rules/platform/platform_model.md — source selection by target.
func TestPanicPlanInvalidation(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".sec", "app", "helper"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for file, fixture := range map[string]string{"helper.sec": "helper_platform", "small.sec": "helper_small", "large.sec": "helper_large"} {
		if err := os.WriteFile(filepath.Join(root, "helper", file), []byte(panicRefreshFixture(t, fixture)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	uri := uriFromPath(filepath.Join(root, "app", "main.sec"))
	source := panicRefreshFixture(t, "main")
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	s.documentSnapshots.Open(uri, 1, source)
	dir := normalizedSourcePath(filepath.Dir(pathFromURI(uri)))
	s.crossTarget = newCrossTargetStore()
	for _, arch := range []string{"amd64", "armv7", "amd64"} {
		manifest := "[project]\nname = \"panic-plan-refresh\"\n[variant.active]\nos = \"linux\"\narch = \"" + arch + "\"\n[target.app]\nkind = \"command\"\nsource = \"app\"\nvariants = [\"active\"]\n"
		if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}
		old := s.diagnosticGeneration[dir]
		s.crossTarget.results[uri] = crossTargetResult{textHash: documentTextHash(source)}
		if err := s.handle(rpcMessage{Method: "workspace/didChangeWatchedFiles", Params: json.RawMessage(`{"changes":[]}`)}); err != nil {
			t.Fatal(err)
		}
		if _, known := s.crossTargetResultFor(uri, source); known {
			t.Fatal("old target's panic facts retained")
		}
		out.Reset()
		if err := s.publishDiagnosticBatch(dir, old); err != nil {
			t.Fatal(err)
		}
		if out.Len() != 0 {
			t.Fatal("old target diagnostics published")
		}
		if err := s.publishDiagnosticBatch(dir, s.diagnosticGeneration[dir]); err != nil {
			t.Fatal(err)
		}
		wantPanic := arch == "armv7"
		if strings.Contains(out.String(), "S1092") != wantPanic {
			t.Fatal(arch, out.String())
		}
		cold := analyze(uri, source, s.sourceOverlay())
		wantCount := 0
		if wantPanic {
			wantCount = 1
		}
		if len(cold) != wantCount {
			t.Fatal(arch, cold)
		}
		hover, ok := hoverForSource(uri, source, offsetPosition(source, strings.Index(source, "Root() void")), s.sourceOverlay())
		if !ok || strings.Contains(hover.Contents.Value, "May panic: `yes`") != wantPanic {
			t.Fatal(arch, hover)
		}
	}
}
