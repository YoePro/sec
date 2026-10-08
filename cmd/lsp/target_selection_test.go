package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sec/internal/lsp/protocol"
	lspserver "sec/internal/lsp/server"
	"strings"
	"testing"
	"time"
)

// targetSelectionFixture supplies source and project inputs from testdata.
// Rules: rules/tooling/lsp.md — Target-aware analysis; projects/projects.md §19.
func targetSelectionFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{".sec", "app"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for file, dest := range map[string]string{"sec.toml": ".sec/sec.toml", "main.sec": "app/main.sec", "small.sec": "app/small.sec", "large.sec": "app/large.sec"} {
		data, err := os.ReadFile("../../testdata/lsp/target_selection/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dest), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "app/main.sec"))
	if err != nil {
		t.Fatal(err)
	}
	return root, uriFromPath(filepath.Join(root, "app/main.sec")), string(data)
}

// TestTargetSelectionProtocol verifies discovery, switching, reset, invalid
// selection preservation, source-directive precedence and immutable requests.
// Rules: rules/tooling/lsp.md — Target-aware analysis, Snapshots, Multi-target diagnostics;
// rules/types/types.md — int and uint, Binary floating-point types.
func TestTargetSelectionProtocol(t *testing.T) {
	root, uri, text := targetSelectionFixture(t)
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	s.documentSnapshots.Open(uri, 1, text)
	request := func(method string, selection lspTargetSelection) protocol.Message {
		t.Helper()
		s.writeMu.Lock()
		out.Reset()
		s.writeMu.Unlock()
		params, _ := json.Marshal(map[string]string{"uri": uri, "logicalTarget": selection.LogicalTarget, "variant": selection.Variant})
		if err := s.handle(rpcMessage{Method: method, ID: json.RawMessage(`1`), Params: params}); err != nil {
			t.Fatal(err)
		}
		s.writeMu.Lock()
		data := append([]byte(nil), out.Bytes()...)
		s.writeMu.Unlock()
		response, err := protocol.ReadMessage(bufio.NewReader(bytes.NewReader(data)))
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	response := request("sec/targets", lspTargetSelection{})
	var options []lspTargetOption
	rawOptions, _ := json.Marshal(response.Result)
	if err := json.Unmarshal(rawOptions, &options); err != nil {
		t.Fatal(err)
	}
	if len(options) != 3 || options[0].PointerWidthBits != 32 || options[1].PointerWidthBits != 64 {
		t.Fatal(options)
	}
	for _, option := range options {
		if option.Active {
			t.Fatal("ambiguous project guessed a variant", options)
		}
	}
	response = request("sec/selectTarget", lspTargetSelection{"app", "small"})
	if response.Error != nil {
		t.Fatal(response.Error)
	}
	small := s.sourceOverlay()
	_, otherURI, otherText := targetSelectionFixture(t)
	if got := newLSPAnalyzerWithOverlay(otherURI, parseProgramForLSP(otherURI, otherText), small).Types()["int"].BitWidth; got != 0 {
		t.Fatal("selection leaked into another project", got)
	}

	program := parseProgramForLSP(uri, text)
	a := newLSPAnalyzerWithOverlay(uri, program, small)
	if a.Types()["int"].BitWidth != 32 || a.Types()["uint"].BitWidth != 32 || a.Types()["float"].FloatBits != 32 {
		t.Fatal(a.Types()["int"])
	}
	assertDiagnosticMessage(t, analyze(uri, text, small), "overflows int")
	dir := normalizedSourcePath(filepath.Join(root, "app"))
	s.timerMu.Lock()
	oldGeneration := s.diagnosticGeneration[dir]
	s.timerMu.Unlock()
	response = request("sec/selectTarget", lspTargetSelection{"app", "large"})
	if response.Error != nil {
		t.Fatal(response.Error)
	}
	large := s.sourceOverlay()
	if got := newLSPAnalyzerWithOverlay(uri, parseProgramForLSP(uri, text), large).Types()["int"].BitWidth; got != 64 {
		t.Fatal(got)
	}
	if got := newLSPAnalyzerWithOverlay(uri, parseProgramForLSP(uri, text), small).Types()["int"].BitWidth; got != 32 {
		t.Fatal("snapshot mutated", got)
	}
	for _, d := range analyze(uri, text, large) {
		if strings.Contains(d.Message, "overflows int") {
			t.Fatal(d)
		}
	}

	s.crossTarget.mu.Lock()
	s.crossTarget.generation[uri]++ // isolate an exact current result from asynchronous scheduling
	s.crossTarget.results[uri] = crossTargetResult{textHash: documentTextHash(text), active: "obsolete", perTarget: map[string][]diagnostic{}}
	s.crossTarget.mu.Unlock()
	// A watched manifest change clears stored comparisons before refreshing.
	if err := s.handle(rpcMessage{Method: "workspace/didChangeWatchedFiles", Params: json.RawMessage(`{"changes":[]}`)}); err != nil {
		t.Fatal(err)
	}
	s.crossTarget.mu.Lock()
	retained, hasRetained := s.crossTarget.results[uri]
	s.crossTarget.mu.Unlock()
	if hasRetained && retained.active == "obsolete" {
		t.Fatal("configuration retained old variant result")
	}
	s.timerMu.Lock()
	newGeneration := s.diagnosticGeneration[dir]
	s.timerMu.Unlock()
	if newGeneration <= oldGeneration {
		t.Fatal("target switch retained old job")
	}
	s.writeMu.Lock()
	out.Reset()
	s.writeMu.Unlock()
	if err := s.publishDiagnosticBatch(dir, oldGeneration); err != nil {
		t.Fatal(err)
	}
	s.writeMu.Lock()
	size := out.Len()
	s.writeMu.Unlock()
	if size != 0 {
		t.Fatal("obsolete target job published")
	}
	response = request("sec/selectTarget", lspTargetSelection{"app", "missing"})
	if response.Error == nil || response.Error.Code != -32602 {
		t.Fatal(response)
	}
	if s.sourceOverlay().Targets[normalizedSourcePath(root)].Variant != "large" {
		t.Fatal("invalid choice replaced active variant")
	}
	directed, err := os.ReadFile("../../testdata/lsp/target_selection/directed.sec")
	if err != nil {
		t.Fatal(err)
	}
	if got := newLSPAnalyzerWithOverlay(uri, parseProgramForLSP(uri, string(directed)), small).Types()["int"].BitWidth; got != 64 {
		t.Fatal("source #target lost precedence", got)
	}
	response = request("sec/selectTarget", lspTargetSelection{})
	if response.Error != nil {
		t.Fatal(response.Error)
	}
	if len(s.sourceOverlay().Targets) != 0 {
		t.Fatal("reset did not restore automatic selection")
	}
}

// TestTargetSelectionVariantsAndFeatures proves named variants survive equal
// platform plans, and completion/batched diagnostics consume the active plan.
// Rules: rules/tooling/lsp.md — Multi-target diagnostics, Completion, Snapshots.
func TestTargetSelectionVariantsAndFeatures(t *testing.T) {
	root, uri, text := targetSelectionFixture(t)
	overlay := sourceOverlay{Targets: map[string]lspTargetSelection{normalizedSourcePath(root): {"app", "large"}}}
	result, ok := computeCrossTargetDiagnostics(uri, text, overlay)
	if !ok || len(result.targets) != 3 {
		t.Fatal(result, ok)
	}
	for _, name := range []string{"app/small (linux-armv7)", "app/duplicate (linux-armv7)"} {
		if len(result.perTarget[name]) != 1 {
			t.Fatal(name, result.perTarget)
		}
	}
	if len(result.perTarget["app/large (linux-amd64)"]) != 0 {
		t.Fatal(result.perTarget)
	}
	merged := mergeCrossTargetDiagnostics(analyze(uri, text, overlay), result)
	if len(merged) != 1 || !strings.Contains(merged[0].Message, "applies to 2 of 3 targets") || !strings.Contains(merged[0].Message, "app/duplicate") {
		t.Fatal(merged)
	}
	source, err := os.ReadFile("../../testdata/lsp/target_selection/completion.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"small", "large"} {
		overlay.Targets[normalizedSourcePath(root)] = lspTargetSelection{"app", variant}
		items := completeSource(uri, string(source), strings.Index(string(source), "value.")+len("value."), overlay)
		found := map[string]bool{}
		for _, item := range items {
			found[item.Label] = true
		}
		if found["Only32"] != (variant == "small") || found["Only64"] != (variant == "large") {
			t.Fatal(variant, items)
		}
		batch := analyzeDiagnosticBatch([]lspserver.Snapshot{{URI: uri, Text: text, Version: 1}}, overlay)
		overflow := false
		for _, d := range batch[uri] {
			overflow = overflow || strings.Contains(d.Message, "overflows int")
		}
		if overflow != (variant == "small") {
			t.Fatal(variant, batch)
		}
	}
}
