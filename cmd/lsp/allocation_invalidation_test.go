package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lspserver "sec/internal/lsp/server"
)

// allocationRefreshFixture loads canonical source fixtures without inline Sec.
// Rules: rules/memory/allocation.md — §29(5), §30.
func allocationRefreshFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/lsp/allocation_refresh/" + name + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestAllocationImportInvalidation exercises body and trusted-contract changes
// through real LSP document events, including stale work and closing an overlay.
// A previously analyzed unrelated module retains its scheduled generation.
// Rules: rules/memory/allocation.md — §29(5); rules/tooling/lsp.md — "Snapshots", "Incremental analysis".
func TestAllocationImportInvalidation(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte("[project]\nname = \"allocation-refresh\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mainFile := filepath.Join(root, "app", "main.sec")
	helperFile := filepath.Join(root, "helper", "helper.sec")
	for _, path := range []string{mainFile, helperFile} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
	}
	mainSource := allocationRefreshFixture(t, "main")
	free := allocationRefreshFixture(t, "helper_free")
	for file, source := range map[string]string{mainFile: mainSource, helperFile: free} {
		if err := os.WriteFile(file, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	mainURI, helperURI := uriFromPath(mainFile), uriFromPath(helperFile)
	unrelatedURI := uriFromPath(filepath.Join(t.TempDir(), "other.sec"))
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	s.documentSnapshots.Open(mainURI, 1, mainSource)
	s.documentSnapshots.Open(helperURI, 1, free)
	s.documentSnapshots.Open(unrelatedURI, 1, free)
	for _, uri := range []string{mainURI, helperURI, unrelatedURI} {
		if err := s.publishModuleDiagnostics(uri); err != nil {
			t.Fatal(err)
		}
	}
	mainDir := normalizedSourcePath(filepath.Dir(mainFile))
	otherDir := normalizedSourcePath(filepath.Dir(pathFromURI(unrelatedURI)))
	if !s.diagnosticDependencies[mainDir][normalizedSourcePath(helperFile)] {
		t.Fatal("import dependency not recorded", s.diagnosticDependencies)
	}
	untouched := s.diagnosticGeneration[otherDir]
	s.crossTarget = newCrossTargetStore()
	s.crossTarget.results[unrelatedURI] = crossTargetResult{textHash: documentTextHash(free)}
	for index, name := range []string{"helper_allocating", "helper_free", "helper_untrusted", "helper_trusted"} {
		old := s.diagnosticGeneration[mainDir]
		s.crossTarget.results[mainURI] = crossTargetResult{textHash: documentTextHash(mainSource)}
		previousCross := s.crossTarget.generation[mainURI]
		source := allocationRefreshFixture(t, name)
		params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": helperURI, "version": index + 2}, "contentChanges": []map[string]string{{"text": source}}})
		if err := s.handle(rpcMessage{Method: "textDocument/didChange", Params: params}); err != nil {
			t.Fatal(err)
		}
		if s.diagnosticGeneration[mainDir] <= old || s.diagnosticGeneration[otherDir] != untouched {
			t.Fatal("dependent invalidation missing or unrelated module refreshed", s.diagnosticGeneration)
		}
		if _, known := s.crossTargetResultFor(mainURI, mainSource); known || s.crossTarget.generation[mainURI] <= previousCross {
			t.Fatal("stale imported cross-plan facts retained")
		}
		if _, known := s.crossTargetResultFor(unrelatedURI, free); !known {
			t.Fatal("unrelated cross-plan facts invalidated")
		}
		out.Reset()
		if err := s.publishDiagnosticBatch(mainDir, old); err != nil {
			t.Fatal(err)
		}
		if out.Len() != 0 {
			t.Fatal("stale allocation result published")
		}
		if err := s.publishDiagnosticBatch(mainDir, s.diagnosticGeneration[mainDir]); err != nil {
			t.Fatal(err)
		}
		want := name == "helper_allocating" || name == "helper_untrusted"
		if strings.Contains(out.String(), "S1108") != want {
			t.Fatal(name, out.String())
		}
		hover, ok := hoverForSource(mainURI, mainSource, offsetPosition(mainSource, strings.Index(mainSource, "Root() void")), s.sourceOverlay())
		if !ok {
			t.Fatal("missing current hover")
		}
		knowledge := "allocation-free"
		if name == "helper_allocating" {
			knowledge = "may-allocate"
		}
		if name == "helper_untrusted" {
			knowledge = "unknown"
		}
		if !strings.Contains(hover.Contents.Value, "Allocation behavior: `"+knowledge+"`") {
			t.Fatal(name, hover)
		}
	}
	// Removing unsaved allocating contents must restore the allocation-free disk body.
	s.documentSnapshots.Change(helperURI, 9, allocationRefreshFixture(t, "helper_allocating"))
	s.scheduleSourceDiagnostics(helperURI)
	if err := s.publishModuleDiagnostics(mainURI); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "S1108") {
		t.Fatal("expected allocating overlay")
	}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": helperURI}})
	if err := s.handle(rpcMessage{Method: "textDocument/didClose", Params: params}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := s.publishDiagnosticBatch(mainDir, s.diagnosticGeneration[mainDir]); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "S1108") {
		t.Fatal("closed overlay's effect survived", out.String())
	}
	// Replacing the import graph removes dependencies on the old module.
	otherFile := filepath.Join(root, "other", "other.sec")
	if err := os.MkdirAll(filepath.Dir(otherFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherFile, []byte(allocationRefreshFixture(t, "other_free")), 0644); err != nil {
		t.Fatal(err)
	}
	otherSource := allocationRefreshFixture(t, "main_other")
	change, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": mainURI, "version": 2}, "contentChanges": []map[string]string{{"text": otherSource}}})
	if err := s.handle(rpcMessage{Method: "textDocument/didChange", Params: change}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := s.publishModuleDiagnostics(mainURI); err != nil {
		t.Fatal(err)
	}
	if s.diagnosticDependencies[mainDir][normalizedSourcePath(helperFile)] || !s.diagnosticDependencies[mainDir][normalizedSourcePath(otherFile)] {
		t.Fatal("obsolete import dependency retained", s.diagnosticDependencies)
	}
	generation := s.diagnosticGeneration[mainDir]
	open, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": helperURI, "version": 10, "text": allocationRefreshFixture(t, "helper_allocating")}})
	if err := s.handle(rpcMessage{Method: "textDocument/didOpen", Params: open}); err != nil {
		t.Fatal(err)
	}
	if s.diagnosticGeneration[mainDir] != generation {
		t.Fatal("removed dependency still invalidated root")
	}
	// A changed closed import is read from disk on the watched-file refresh.
	if err := os.WriteFile(otherFile, []byte(allocationRefreshFixture(t, "other_allocating")), 0644); err != nil {
		t.Fatal(err)
	}
	watched, _ := json.Marshal(map[string]any{"changes": []map[string]any{{"uri": uriFromPath(otherFile), "type": 2}}})
	if err := s.handle(rpcMessage{Method: "workspace/didChangeWatchedFiles", Params: watched}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := s.publishDiagnosticBatch(mainDir, generation); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatal("closed import's previous summary published")
	}
	if err := s.publishDiagnosticBatch(mainDir, s.diagnosticGeneration[mainDir]); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "S1108") {
		t.Fatal("closed import change did not update allocation", out.String())
	}
}

// TestAllocationProfileInvalidation exercises the real manifest target plan:
// hosted materialization becomes unavailable under the represented freestanding
// plan and returns on reload, while older queued work is invalidated immediately.
// Rules: rules/memory/allocation.md — §§17(7)-(9),22(4),29(5).
func TestAllocationProfileInvalidation(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, ".sec")
	if err := os.Mkdir(manifestDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(manifestDir, "sec.toml")
	sourceFile := filepath.Join(root, "app", "main.sec")
	data, err := os.ReadFile("../../testdata/sema/allocation_context_facts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(sourceFile)
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	s.documentSnapshots.Open(uri, 1, source)
	dir := normalizedSourcePath(filepath.Dir(sourceFile))
	for _, target := range []struct{ os, arch, profile string }{{"linux", "amd64", "hosted"}, {"baremetal", "cortex-m3", "freestanding"}, {"linux", "amd64", "hosted"}} {
		content := "[project]\nname = \"allocation-profile\"\n[variant.active]\nos = \"" + target.os + "\"\narch = \"" + target.arch + "\"\n[target.app]\nkind = \"command\"\nsource = \"app\"\nvariants = [\"active\"]\n"
		if err := os.WriteFile(manifest, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		previous := s.diagnosticGeneration[dir]
		if err := s.handle(rpcMessage{Method: "workspace/didChangeWatchedFiles", Params: json.RawMessage(`{"changes":[]}`)}); err != nil {
			t.Fatal(err)
		}
		out.Reset()
		if err := s.publishDiagnosticBatch(dir, previous); err != nil {
			t.Fatal(err)
		}
		if out.Len() != 0 {
			t.Fatal("old profile job published")
		}
		if err := s.publishDiagnosticBatch(dir, s.diagnosticGeneration[dir]); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "S1098") != (target.profile == "freestanding") {
			t.Fatal(target, out.String())
		}
		hover, ok := hoverForSource(uri, source, offsetPosition(source, strings.Index(source, "Render(name")))
		if !ok || !strings.Contains(hover.Contents.Value, "profile "+target.profile) {
			t.Fatal(target, hover)
		}
	}
}
