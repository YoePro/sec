package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	lspserver "sec/internal/lsp/server"
)

// typingTargetServer uses real compiler sources but manually releases the
// hour-long debounce so synchronization tests need no timing-based sleeps.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Document snapshots.
func typingTargetServer(t *testing.T) (*server, string, string, string) {
	t.Helper()
	uri, source := targetAnalysisFixture(t)
	edited := targetAnalysisSource(t, "native_edited.sec")
	s := &server{out: &bytes.Buffer{}, documentSnapshots: lspserver.NewDocuments(), crossTarget: newCrossTargetStore(), diagnosticDelay: time.Hour}
	t.Cleanup(s.stopDiagnosticTimers)
	return s, uri, source, edited
}

func typingTargetNotify(t *testing.T, s *server, method, uri, text string, version int) {
	t.Helper()
	document := map[string]any{"uri": uri, "version": version}
	params := map[string]any{"textDocument": document}
	if method == "textDocument/didOpen" {
		document["text"] = text
	}
	if method == "textDocument/didChange" {
		params["contentChanges"] = []map[string]any{{"text": text}}
	}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.handle(rpcMessage{Method: method, Params: data}); err != nil {
		t.Fatal(err)
	}
}

func typingTargetJob(t *testing.T, s *server, uri string) (*crossTargetWork, uint64) {
	t.Helper()
	s.crossTarget.mu.Lock()
	defer s.crossTarget.mu.Unlock()
	job := s.crossTarget.work[uri]
	if job == nil || job.pending == nil {
		t.Fatal("typing did not schedule a target comparison")
	}
	job.timer.Stop()
	return job, s.crossTarget.generation[uri]
}

// Successive unsaved versions coalesce; obsolete callbacks cannot run Sema,
// and the comparison/status/publication belongs to the latest editor text.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Document synchronization;
// rules/compiler/incremental_compilation.md — §§4,7,13.
func TestCrossTargetTypingCoalescesLatestVersion(t *testing.T) {
	s, uri, source, edited := typingTargetServer(t)
	typingTargetNotify(t, s, "textDocument/didOpen", uri, source, 1)
	old, oldGeneration := typingTargetJob(t, s, uri)
	typingTargetNotify(t, s, "textDocument/didChange", uri, edited, 2)
	typingTargetNotify(t, s, "textDocument/didChange", uri, edited+"\n", 3)
	job, generation := typingTargetJob(t, s, uri)
	s.crossTarget.mu.Lock()
	pendingSlots := len(s.crossTarget.work)
	s.crossTarget.mu.Unlock()
	if pendingSlots != 1 || generation <= oldGeneration {
		t.Fatal("versions did not coalesce into one pending slot")
	}
	before := targetAnalysisCount()
	s.runCrossTargetWork(uri, old, oldGeneration)
	if targetAnalysisCount() != before {
		t.Fatal("superseded timer ran compiler analysis")
	}
	s.runCrossTargetWork(uri, job, generation)
	status := s.targetStatusFor(uri, nil)
	if status.State != "current" || status.Version != 3 || len(status.Targets) != 2 {
		t.Fatal(status)
	}
	for _, row := range status.Targets {
		if len(row.Diagnostics) != 0 {
			t.Fatal(row.Diagnostics)
		}
	}
	disk, err := os.ReadFile(pathFromURI(uri))
	if err != nil || string(disk) != source {
		t.Fatal("comparison required saving the editor buffer")
	}
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(s.out.(*bytes.Buffer).Bytes(), []byte(`"version":3`)) {
		t.Fatal("latest comparison was not published with the editor version")
	}
}

// An edited imported source invalidates comparisons whose own text did not
// change. The new comparison consumes the same immutable unsaved overlay.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Incremental analysis;
// rules/compiler/incremental_compilation.md — §§11–13.
func TestCrossTargetTypingRefreshesDependencies(t *testing.T) {
	s, uri, _, edited := typingTargetServer(t)
	typingTargetNotify(t, s, "textDocument/didOpen", uri, edited, 1)
	job, generation := typingTargetJob(t, s, uri)
	s.runCrossTargetWork(uri, job, generation)
	if err := s.publishModuleDiagnostics(uri); err != nil {
		t.Fatal(err)
	}
	dependency := filepath.Join(filepath.Dir(pathFromURI(uri)), "dep", "value.sec")
	dependencyURI := uriFromPath(dependency)
	original := targetAnalysisSource(t, "dep/value.sec")
	typingTargetNotify(t, s, "textDocument/didOpen", dependencyURI, original, 1)
	changed := targetAnalysisSource(t, "value_changed.sec")
	typingTargetNotify(t, s, "textDocument/didChange", dependencyURI, changed, 2)
	if status := s.targetStatusFor(uri, nil); status.State != "unavailable" {
		t.Fatal("obsolete dependent comparison stayed current")
	}
	job, generation = typingTargetJob(t, s, uri)
	s.runCrossTargetWork(uri, job, generation)
	status := s.targetStatusFor(uri, nil)
	if status.State != "current" || status.Version != 1 {
		t.Fatal(status)
	}
	for _, row := range status.Targets {
		if len(row.Diagnostics) == 0 {
			t.Fatal("unsaved dependency error missing from target", row.Name)
		}
	}
}

// One in-flight comparison retains just the newest pending text. Cache locking
// supplies a deterministic barrier in the actual compiler path, not a second
// analysis producer. Close/shutdown withdraw all pending callbacks.
// Rules: rules/tooling/lsp.md — Document snapshots, Responsiveness model;
// rules/compiler/incremental_compilation.md — §§4,7.
func TestCrossTargetTypingSerializesAndCancels(t *testing.T) {
	s, uri, source, edited := typingTargetServer(t)
	typingTargetNotify(t, s, "textDocument/didOpen", uri, source, 1)
	job, generation := typingTargetJob(t, s, uri)
	sharedTargetAnalyses.mu.Lock()
	var unlock sync.Once
	release := func() { unlock.Do(sharedTargetAnalyses.mu.Unlock) }
	defer release()
	done := make(chan struct{})
	go func() { defer close(done); s.runCrossTargetWork(uri, job, generation) }()
	deadline := time.After(10 * time.Second)
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		s.crossTarget.mu.Lock()
		running := job.running
		s.crossTarget.mu.Unlock()
		if running {
			break
		}
		select {
		case <-tick.C:
		case <-deadline:
			t.Fatal("worker did not start")
		}
	}
	typingTargetNotify(t, s, "textDocument/didChange", uri, edited, 2)
	next, nextGeneration := typingTargetJob(t, s, uri)
	s.runCrossTargetWork(uri, next, nextGeneration)
	s.crossTarget.mu.Lock()
	retained := next == job && job.running && job.ready && job.pending != nil && job.pending.text == edited
	s.crossTarget.mu.Unlock()
	if !retained {
		t.Fatal("another worker consumed the pending request")
	}
	release()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("worker did not finish")
	}
	if status := s.targetStatusFor(uri, nil); status.State != "current" || status.Version != 2 {
		t.Fatal(status)
	}
	typingTargetNotify(t, s, "textDocument/didChange", uri, edited+"\n", 3)
	pending, pendingGeneration := typingTargetJob(t, s, uri)
	typingTargetNotify(t, s, "textDocument/didClose", uri, "", 0)
	s.runCrossTargetWork(uri, pending, pendingGeneration)
	if _, ok := s.crossTargetResultFor(uri, edited+"\n"); ok {
		t.Fatal("closed document gained a comparison")
	}
	typingTargetNotify(t, s, "textDocument/didOpen", uri, edited, 4)
	pending, pendingGeneration = typingTargetJob(t, s, uri)
	s.stopDiagnosticTimers()
	s.runCrossTargetWork(uri, pending, pendingGeneration)
	s.crossTarget.mu.Lock()
	pendingCount := len(s.crossTarget.work)
	s.crossTarget.mu.Unlock()
	if pendingCount != 0 {
		t.Fatal("shutdown retained target work")
	}
}
