package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	lspserver "sec/internal/lsp/server"
)

// One document state is analyzed once for diagnostics and requests together:
// a single-document diagnostic group and a later request for the same text,
// open documents and targets share one analyzer, the shared diagnostics equal
// an independent analysis, and a change to another open document (a possible
// import) or an outside-identity message yields a fresh analysis. A group of
// several documents keeps its own joint analysis.
// Rules: rules/tooling/lsp.md — "Responsiveness model", "Snapshots".
func TestDiagnosticsAndRequestsShareOneAnalysis(t *testing.T) {
	enableAnalysisCache()
	defer disableAnalysisCache()
	source, err := os.ReadFile("../../testdata/module_siblings/completion_types.sec")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	sibling := filepath.Join(dir, "types.sec")
	path := filepath.Join(dir, "main.sec")
	text := "module module_siblings\nfn Check() AuthError { return AuthError.Invalid }\n"
	overlay := sourceOverlay{Sources: map[string]string{sibling: string(source), path: text}}
	snapshot := lspserver.Snapshot{URI: uriFromPath(path), Version: 1, Text: text}

	results := analyzeDiagnosticBatch([]lspserver.Snapshot{snapshot}, overlay)
	_, first, release := cachedLSPAnalysis(snapshot.URI, text, parseProgramForLSP(snapshot.URI, text), overlay)
	release()
	_, again, release := cachedLSPAnalysis(snapshot.URI, text, parseProgramForLSP(snapshot.URI, text), overlay)
	release()
	if first != again {
		t.Fatal("a repeated request for the same document state analyzed again")
	}

	disableAnalysisCache()
	want := analyze(snapshot.URI, text, overlay)
	enableAnalysisCache()
	if !reflect.DeepEqual(results[snapshot.URI], want) {
		t.Fatalf("shared diagnostics = %+v, independent = %+v", results[snapshot.URI], want)
	}

	// The diagnostics' analysis is the one a request then reuses.
	analyzeDiagnosticBatch([]lspserver.Snapshot{snapshot}, overlay)
	_, fromDiagnostics, release := cachedLSPAnalysis(snapshot.URI, text, parseProgramForLSP(snapshot.URI, text), overlay)
	release()
	_, reused, release := cachedLSPAnalysis(snapshot.URI, text, parseProgramForLSP(snapshot.URI, text), overlay)
	release()
	if fromDiagnostics != reused {
		t.Fatal("a request after diagnostics did not reuse their analysis")
	}

	edited := sourceOverlay{Sources: map[string]string{sibling: string(source) + "\n// edited\n", path: text}}
	_, afterImportEdit, release := cachedLSPAnalysis(snapshot.URI, text, parseProgramForLSP(snapshot.URI, text), edited)
	release()
	if afterImportEdit == reused {
		t.Fatal("an edit to another open document reused a stale analysis")
	}

	advanceAnalysisEpoch()
	_, afterEpoch, release := cachedLSPAnalysis(snapshot.URI, text, parseProgramForLSP(snapshot.URI, text), edited)
	release()
	if afterEpoch == afterImportEdit {
		t.Fatal("a change outside the identity reused a stale analysis")
	}

	// A multi-document group still matches individual analysis.
	snapshots, groupOverlay := diagnosticBatchFixture(t, 2)
	group := analyzeDiagnosticBatch(snapshots, groupOverlay)
	disableAnalysisCache()
	for _, snapshot := range snapshots {
		if want := analyze(snapshot.URI, snapshot.Text, groupOverlay); !reflect.DeepEqual(group[snapshot.URI], want) {
			t.Fatalf("%s: group = %+v, individual = %+v", snapshot.URI, group[snapshot.URI], want)
		}
	}
	enableAnalysisCache()
}
