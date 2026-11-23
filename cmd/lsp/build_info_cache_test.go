package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// An unchanged Sec document cannot reuse metadata from a different snapshot,
// even before a disk watcher arrives. Equivalent metadata and unrelated TOML
// comments preserve the entry; missing/restored metadata invalidate as well.
// Rules: rules/compiler/compiler_known_members.md — §113B(3–4);
// rules/projects/projects.md — §§33–34;
// rules/compiler/incremental_compilation.md — §§4,7–8.
func TestBuildInfoSemanticCacheInvalidation(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, ".sec", "sec.toml")
	sourcePath := filepath.Join(root, "cmd", "app", "main.sec")
	manifest, err := os.ReadFile("../../testdata/platform/buildinfo/source_facts.toml")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/lsp/build_info_cache/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{manifestPath, sourcePath} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(sourcePath, source, 0600); err != nil {
		t.Fatal(err)
	}
	writeManifest := func(text string) {
		t.Helper()
		if err := os.WriteFile(manifestPath, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest(string(manifest))
	enableAnalysisCache()
	t.Cleanup(disableAnalysisCache)
	uri := uriFromPath(sourcePath)
	overlay := sourceOverlay{Sources: map[string]string{normalizedSourcePath(sourcePath): string(source)}}
	analyze := func(wantBuild uint64, available bool) *cachedAnalysis {
		t.Helper()
		p := parser.New(lexer.NewWithFile(string(source), sourcePath))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		result, release := cachedLSPAnalysisResult(uri, string(source), program, overlay)
		info, ok := result.analyzer.BuildInfo()
		errors := append([]string(nil), p.Errors()...)
		for _, err := range result.semanticErrors {
			errors = append(errors, err.Message)
		}
		release()
		if ok != available || ok && info.VersionBuild != wantBuild {
			t.Fatalf("snapshot = %+v (%v), want %d (%v)", info, ok, wantBuild, available)
		}
		if available && len(errors) != 0 {
			t.Fatal(errors)
		}
		return completionCacheEntry(t, uri)
	}
	first := analyze(7, true)
	if analyze(7, true) != first {
		t.Fatal("identical metadata did not reuse analysis")
	}
	writeManifest(string(manifest) + "\n# unrelated comment\n")
	if analyze(7, true) != first {
		t.Fatal("metadata key depends on raw manifest spelling")
	}
	changed := strings.Replace(string(manifest), "build = 7", "build = 8", 1)
	writeManifest(changed)
	second := analyze(8, true)
	if second == first || second.buildInfoKey == first.buildInfoKey {
		t.Fatal("observable metadata change reused old analysis")
	}
	writeManifest(strings.Replace(changed, "profile = 'release'", "profile = 'debug'", 1))
	third := analyze(8, true)
	if third == second || third.buildInfoKey == second.buildInfoKey {
		t.Fatal("profile change reused old metadata")
	}
	writeManifest("not valid TOML [")
	missing := analyze(0, false)
	if missing == third {
		t.Fatal("unavailable metadata reused positive result")
	}
	writeManifest(string(manifest))
	if analyze(7, true) == missing {
		t.Fatal("restored metadata reused unavailable snapshot")
	}
}
