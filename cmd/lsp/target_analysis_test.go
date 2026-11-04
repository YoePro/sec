package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	platformtarget "sec/internal/platform/target"
)

func targetAnalysisFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"native.sec", "linux.sec", "windows.sec", "dep/value.sec"} {
		writeWorkspaceSource(t, root, name, targetAnalysisSource(t, name))
	}
	return uriFromPath(filepath.Join(root, "native.sec")), targetAnalysisSource(t, "native.sec")
}

func targetAnalysisSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/lsp/shared_target_analysis/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func targetAnalysisCount() uint64 {
	sharedTargetAnalyses.mu.Lock()
	defer sharedTargetAnalyses.mu.Unlock()
	return sharedTargetAnalyses.computations
}

// Both real consumers reuse exactly one Sema run per effective target. Their
// projections retain conformance failures and target-width diagnostics.
// Rules: rules/tooling/lsp.md — "Multi-target diagnostics", "Interface conformance code lens".
func TestSharedTargetAnalysisAcrossConsumers(t *testing.T) {
	for _, first := range []string{"diagnostics", "lens", "concurrent"} {
		t.Run(first, func(t *testing.T) {
			uri, text := targetAnalysisFixture(t)
			overlay := sourceOverlay{}
			before := targetAnalysisCount()
			diagnostics := func() {
				result, ok := computeCrossTargetDiagnostics(uri, text, overlay)
				if !ok || len(result.targets) != 2 || len(result.perTarget["linux-amd64"]) != 0 || len(result.perTarget["windows-386"]) == 0 {
					t.Errorf("target diagnostic projection: %+v, %t", result, ok)
				}
			}
			lens := func() {
				summaries := crossTargetConformance(uri, text, overlay)
				if len(summaries) != 1 || len(summaries[0].targets) != 2 {
					t.Errorf("target conformance projection: %+v", summaries)
					return
				}
				if title := interfaceConformanceCommand(uri, summaries[0]).Title; !strings.Contains(title, "1 of 2") || !strings.Contains(title, "windows-386: Size") {
					t.Error(title)
				}
			}
			switch first {
			case "diagnostics":
				diagnostics()
				lens()
			case "lens":
				lens()
				diagnostics()
			default:
				var done sync.WaitGroup
				done.Add(2)
				go func() { defer done.Done(); diagnostics() }()
				go func() { defer done.Done(); lens() }()
				done.Wait()
			}
			if count := targetAnalysisCount() - before; count != 2 {
				t.Fatalf("ran Sema %d times, want one per target", count)
			}
		})
	}
}

// Facts must invalidate on every prepared-source/configuration change, and
// mutable nested data returned to one consumer cannot corrupt another.
// Rules: rules/tooling/lsp.md — "Document snapshots", "Target-aware analysis".
func TestSharedTargetAnalysisSnapshotsAndInvalidation(t *testing.T) {
	uri, text := targetAnalysisFixture(t)
	root := filepath.Dir(pathFromURI(uri))
	target := platformtarget.Target{OS: "windows", Arch: "386"}
	load := func(overlay sourceOverlay) targetAnalysisSnapshot {
		t.Helper()
		snapshot, err := analysisForTarget(uri, text, overlay, target)
		if err != nil || snapshot.key == "" || len(snapshot.Conformances) == 0 || len(snapshot.Errors) == 0 {
			t.Fatalf("missing snapshot: %+v, %v", snapshot, err)
		}
		return snapshot
	}
	original := load(sourceOverlay{})
	wantError := original.Errors[0].Message
	original.Errors[0].Message = "corrupted"
	original.Conformances[0].Failures[0].Diagnostic.Message = "corrupted"
	if again := load(sourceOverlay{}); again.Errors[0].Message != wantError || again.Conformances[0].Failures[0].Diagnostic.Message == "corrupted" {
		t.Fatal("consumer mutated cached facts")
	}
	dependency := filepath.Join(root, "dep", "value.sec")
	changed := targetAnalysisSource(t, "dep/changed.sec")
	overlay := sourceOverlay{Sources: map[string]string{normalizedSourcePath(dependency): changed}}
	if updated := load(overlay); updated.key == original.key || len(updated.Errors) <= len(original.Errors) {
		t.Fatal("unsaved imported source reused old facts")
	}
	if err := os.WriteFile(dependency, []byte(changed), 0644); err != nil {
		t.Fatal(err)
	}
	if updated := load(sourceOverlay{}); updated.key == original.key {
		t.Fatal("on-disk imported source reused old facts")
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	beforeManifest := load(sourceOverlay{})
	overlay = sourceOverlay{Sources: map[string]string{normalizedSourcePath(manifest): targetAnalysisSource(t, "sec.toml")}}
	if updated := load(overlay); updated.key == beforeManifest.key {
		t.Fatal("unsaved manifest reused old facts")
	}
	beforeEpoch := load(overlay)
	selected := sourceOverlay{Targets: map[string]lspTargetSelection{root: {LogicalTarget: "app", Variant: "small"}}}
	if updated := load(selected); updated.key == beforeManifest.key {
		t.Fatal("explicit selection reused another selection's facts")
	}
	advanceAnalysisEpoch()
	if updated := load(overlay); updated.key == beforeEpoch.key {
		t.Fatal("new analysis epoch reused old facts")
	}
	// Completed lens projections must invalidate too, not hide new target facts.
	crossTargetConformance(uri, text, sourceOverlay{})
	windows := filepath.Join(root, "windows.sec")
	overlay = sourceOverlay{Sources: map[string]string{normalizedSourcePath(windows): targetAnalysisSource(t, "windows_fixed.sec")}}
	summaries := crossTargetConformance(uri, text, overlay)
	if len(summaries) != 1 || !strings.Contains(interfaceConformanceCommand(uri, summaries[0]).Title, "conforms on 2 targets") {
		t.Fatalf("stale conformance presentation: %+v", summaries)
	}
}

// Cache bounds and failure paths are independent from Sema's mutable state.
// No failed supplier may strand waiters or become a successful empty result.
// Rules: rules/tooling/lsp.md — "Responsiveness model", "Document snapshots".
func TestTargetSnapshotCacheCoalescesBoundsAndRetries(t *testing.T) {
	var cache targetSnapshotCache
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	var done sync.WaitGroup
	for i := 0; i < 8; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			data, err := cache.load("shared", func() ([]byte, error) {
				calls.Add(1)
				close(started)
				<-release
				return []byte("snapshot"), nil
			})
			if err != nil || string(data) != "snapshot" {
				t.Error(string(data), err)
			}
		}()
	}
	<-started
	close(release)
	done.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate computation", calls.Load())
	}
	for _, failure := range []string{"error", "panic"} {
		if _, err := cache.load(failure, func() ([]byte, error) {
			if failure == "panic" {
				panic("failed supplier")
			}
			return nil, fmt.Errorf("failed supplier")
		}); err == nil {
			t.Fatal("failure was cached as success")
		}
		if data, err := cache.load(failure, func() ([]byte, error) { return []byte("retry"), nil }); err != nil || string(data) != "retry" {
			t.Fatal("failed supplier was not retried", err)
		}
	}
	for i := 0; i < 40; i++ {
		_, _ = cache.load(fmt.Sprint(i), func() ([]byte, error) { return []byte("small"), nil })
	}
	if len(cache.entries) != 32 {
		t.Fatal("entry budget exceeded", len(cache.entries))
	}
	for i := 0; i < 3; i++ {
		_, _ = cache.load(fmt.Sprint("large", i), func() ([]byte, error) { return bytes.Repeat([]byte{'x'}, 3<<20), nil })
	}
	if cache.bytes > 8<<20 || len(cache.entries) != 2 {
		t.Fatal("byte budget exceeded", cache.bytes, len(cache.entries))
	}
	_, _ = cache.load("oversized", func() ([]byte, error) { return make([]byte, 9<<20), nil })
	if _, stored := cache.entries["oversized"]; stored || cache.bytes > 8<<20 {
		t.Fatal("oversized snapshot retained")
	}
}

// Failure coalescing must release every reader, including after a supplier panic.
// Rules: rules/tooling/lsp.md — "Document snapshots", "Responsiveness model".
func TestTargetSnapshotFailureReleasesWaiters(t *testing.T) {
	var cache targetSnapshotCache
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var done sync.WaitGroup
	for i := 0; i < 8; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			if _, err := cache.load("failure", func() ([]byte, error) {
				once.Do(func() { close(started) })
				<-release
				panic("supplier failed")
			}); err == nil {
				t.Error("failed supplier became successful")
			}
		}()
	}
	<-started
	close(release)
	done.Wait()
	if len(cache.flights) != 0 || len(cache.entries) != 0 {
		t.Fatal("failed flight was retained")
	}
	if _, err := cache.load("failure", func() ([]byte, error) { return []byte("retry"), nil }); err != nil {
		t.Fatal("failure blocked retry", err)
	}
}
