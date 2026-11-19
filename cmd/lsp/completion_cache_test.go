package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// sharedCompletionProject keeps real source in testdata and gives each test
// an isolated source universe for cache epoch and unopened-dependency changes.
// Rules: rules/tooling/lsp.md — "Snapshots", "Completion".
func sharedCompletionProject(t *testing.T) (uri, source, declarations, declarationPath string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"main.sec", "types.sec"} {
		data, err := os.ReadFile("../../testdata/lsp/shared_analysis_cache/" + name)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
		if name == "main.sec" {
			uri, source = uriFromPath(path), string(data)
		} else {
			declarations, declarationPath = string(data), path
		}
	}
	enableAnalysisCache()
	t.Cleanup(disableAnalysisCache)
	return
}

// completionCacheEntry inspects the existing shared service after a request has
// released its read lease; tests do not install a second analysis producer.
func completionCacheEntry(t *testing.T, uri string) *cachedAnalysis {
	t.Helper()
	analysisCache.Lock()
	defer analysisCache.Unlock()
	entry := analysisCache.entries[uri]
	if entry == nil || !entry.ready {
		t.Fatal("completion did not publish a shared analysis")
	}
	return entry
}

// TestCompletionSharesCompilerSnapshot verifies warm cross-feature reuse and
// active AST identity, then changes an unopened dependency without delivering
// its watched-file epoch yet. A private completion analysis would observe that
// file prematurely; the shared snapshot must retain the captured facts.
// Rules: rules/tooling/lsp.md — "Snapshots", "Shared compiler workspace",
// "Responsiveness model", "Completion".
func TestCompletionSharesCompilerSnapshot(t *testing.T) {
	uri, source, declarations, declarationPath := sharedCompletionProject(t)
	member := strings.Index(source, "item.Value") + len("item.")
	pos := offsetPosition(source, member+1)
	if _, ok := hoverForSource(uri, source, pos); !ok {
		t.Fatal("warm hover unavailable")
	}
	entry := completionCacheEntry(t, uri)
	snapshot := func() []byte {
		t.Helper()
		data, err := json.Marshal([]any{entry.sourceProgram, entry.analyzer.Types(), entry.analyzer.Functions(), entry.semanticErrors, entry.analyzer.Warnings(), entry.analyzer.CallGraph().Nodes()})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	before := snapshot()
	if len(entry.sourceProgram.Statements) != 3 || len(entry.program.Statements) <= len(entry.sourceProgram.Statements) {
		t.Fatal("active source roots were confused with assembled module roots")
	}
	// The watcher has not advanced the epoch yet: every feature must still
	// observe the same captured declaration, not a second private analysis.
	changed := strings.Replace(declarations, "Value: int", "Version: int", 1)
	if err := os.WriteFile(declarationPath, []byte(changed), 0644); err != nil {
		t.Fatal(err)
	}
	assertCompletionLabels(t, completeSource(uri, source, member+1), []string{"Value"})
	if definitions := definitionsForSource(uri, source, pos); len(definitions) != 1 {
		t.Fatalf("shared definition missing: %+v", definitions)
	}
	if tokens := semanticTokensForSource(uri, source); len(tokens.Data) == 0 {
		t.Fatal("shared semantic tokens missing")
	}
	if completionCacheEntry(t, uri) != entry || !bytes.Equal(before, snapshot()) {
		t.Fatal("read-only features replaced or mutated their compiler snapshot")
	}
	advanceAnalysisEpoch()
	items := completeSource(uri, source, member+1)
	assertCompletionLabels(t, items, []string{"Version"})
	for _, item := range items {
		if item.Label == "Value" {
			t.Fatal("watched dependency invalidation retained stale members")
		}
	}
	if next := completionCacheEntry(t, uri); next == entry || next.analyzer == entry.analyzer {
		t.Fatal("new epoch reused stale analysis")
	}
}

// TestCompletionRecoveryCacheIdentity keeps synthetic subject/member recovery
// facts distinct from raw document facts, while repeated identical requests
// reuse exactly the analyzed AST from their matching recovery snapshot.
// Rules: rules/tooling/lsp.md — "Recovery nodes", "Snapshots", "Completion".
func TestCompletionRecoveryCacheIdentity(t *testing.T) {
	uri, source, _, _ := sharedCompletionProject(t)
	for _, test := range []struct {
		old, replacement string
		want             []string
	}{
		{"item.Value", "item.", []string{"Value"}},
		{"mode == Mode.First", "mode == ", []string{"Mode.First", "Mode.Second"}},
	} {
		t.Run(test.replacement, func(t *testing.T) {
			text := strings.Replace(source, test.old, test.replacement, 1)
			at := strings.Index(text, test.replacement) + len(test.replacement)
			assertCompletionLabels(t, completeSource(uri, text, at), test.want)
			first := completionCacheEntry(t, uri)
			if first.text == text {
				t.Fatal("recovery facts were labelled as raw source")
			}
			assertCompletionLabels(t, completeSource(uri, text, at), test.want)
			if second := completionCacheEntry(t, uri); second != first || second.analyzer != first.analyzer {
				t.Fatal("identical completion reanalyzed instead of sharing")
			}
			semanticTokensForSource(uri, text)
			raw := completionCacheEntry(t, uri)
			if raw == first || raw.text != text {
				t.Fatal("raw request reused synthetic completion facts")
			}
			assertCompletionLabels(t, completeSource(uri, text, at), test.want)
			if next := completionCacheEntry(t, uri); next == raw || next.text != first.text {
				t.Fatal("completion reused raw rather than matching recovery text")
			}
		})
	}
}

// TestCompletionConcurrentSnapshotReaders exercises the same production read
// lease for completion, hover, navigation and semantic tokens under race tests.
// Rules: rules/tooling/lsp.md — "Snapshots", "Read-only request behavior".
func TestCompletionConcurrentSnapshotReaders(t *testing.T) {
	uri, source, _, _ := sharedCompletionProject(t)
	member := strings.Index(source, "item.Value") + len("item.")
	pos := offsetPosition(source, member+1)
	assertCompletionLabels(t, completeSource(uri, source, member+1), []string{"Value"})
	first := completionCacheEntry(t, uri)
	var jobs sync.WaitGroup
	for i := 0; i < 4; i++ {
		jobs.Add(4)
		go func() { defer jobs.Done(); completeSource(uri, source, member+1) }()
		go func() { defer jobs.Done(); hoverForSource(uri, source, pos) }()
		go func() { defer jobs.Done(); definitionsForSource(uri, source, pos) }()
		go func() { defer jobs.Done(); semanticTokensForSource(uri, source) }()
	}
	jobs.Wait()
	if next := completionCacheEntry(t, uri); next != first || next.analyzer != first.analyzer {
		t.Fatal("concurrent readers did not share one analysis")
	}
}
