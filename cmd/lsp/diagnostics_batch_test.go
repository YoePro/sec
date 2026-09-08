package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	lspserver "sec/internal/lsp/server"
)

func diagnosticBatchFixture(t testing.TB, count int) ([]lspserver.Snapshot, sourceOverlay) {
	t.Helper()
	source, err := os.ReadFile("../../testdata/module_siblings/completion_types.sec")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	overlay := sourceOverlay{filepath.Join(dir, "types.sec"): string(source)}
	snapshots := []lspserver.Snapshot{}
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file%d.sec", i))
		text := fmt.Sprintf("module module_siblings\nfn Check%d() AuthError { return AuthError.Invalid }\nfn Pending%d() void\n", i, i)
		overlay[path] = text
		snapshots = append(snapshots, lspserver.Snapshot{URI: uriFromPath(path), Version: 1, Text: text})
	}
	return snapshots, overlay
}

func TestDiagnosticBatchMatchesIndividualAnalysis(t *testing.T) {
	snapshots, overlay := diagnosticBatchFixture(t, 4)
	results := analyzeDiagnosticBatch(snapshots, overlay)
	for _, snapshot := range snapshots {
		want := analyze(snapshot.URI, snapshot.Text, overlay)
		if !reflect.DeepEqual(results[snapshot.URI], want) {
			t.Fatalf("%s: batch = %+v, individual = %+v", snapshot.URI, results[snapshot.URI], want)
		}
	}
}

func TestModuleDiagnosticsCoalesceAndRejectObsoleteJobs(t *testing.T) {
	snapshots, _ := diagnosticBatchFixture(t, 3)
	var out bytes.Buffer
	s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
	defer s.stopDiagnosticTimers()
	for _, snapshot := range snapshots {
		s.documentSnapshots.Open(snapshot.URI, snapshot.Version, snapshot.Text)
		s.scheduleModuleDiagnostics(snapshot.URI)
	}
	dir := normalizedSourcePath(filepath.Dir(pathFromURI(snapshots[0].URI)))
	if len(s.diagnosticTimers) != 1 {
		t.Fatalf("got %d jobs, want one module job", len(s.diagnosticTimers))
	}
	generation := s.diagnosticGeneration[dir]
	if err := s.publishDiagnosticBatch(dir, generation-1); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatal("obsolete job published diagnostics")
	}
	if err := s.publishDiagnosticBatch(dir, generation); err != nil {
		t.Fatal(err)
	}
	if bytes.Count(out.Bytes(), []byte(`"textDocument/publishDiagnostics"`)) != len(snapshots) {
		t.Fatalf("missing module publications: %s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(`"version":1`)) {
		t.Fatal("missing snapshot version")
	}
	s.stopDiagnosticTimers()
	out.Reset()
	if err := s.publishDiagnosticBatch(dir, generation); err != nil {
		t.Fatal(err)
	}
	s.scheduleModuleDiagnostics(snapshots[0].URI)
	if out.Len() != 0 || len(s.diagnosticTimers) != 0 {
		t.Fatal("shutdown retained diagnostic work")
	}
}

func TestDiagnosticBatchKeepsForeignModuleSeparate(t *testing.T) {
	dir := t.TempDir()
	snapshots := []lspserver.Snapshot{
		{URI: uriFromPath(filepath.Join(dir, "a.sec")), Text: "module a\nenum OnlyA { Yes }\n"},
		{URI: uriFromPath(filepath.Join(dir, "b.sec")), Text: "module b\nfn Read() OnlyA { return OnlyA.Yes }\n"},
	}
	overlay := sourceOverlay{}
	for _, snapshot := range snapshots {
		overlay[pathFromURI(snapshot.URI)] = snapshot.Text
	}
	results := analyzeDiagnosticBatch(snapshots, overlay)
	encoded, _ := json.Marshal(results[snapshots[1].URI])
	if !bytes.Contains(encoded, []byte("unknown type OnlyA")) {
		t.Fatalf("foreign module leaked: %s", encoded)
	}
}

func BenchmarkModuleDiagnostics(b *testing.B) {
	snapshots, overlay := diagnosticBatchFixture(b, 16)
	b.Run("individual", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, snapshot := range snapshots {
				analyze(snapshot.URI, snapshot.Text, overlay)
			}
		}
	})
	b.Run("batch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			analyzeDiagnosticBatch(snapshots, overlay)
		}
	})
}
