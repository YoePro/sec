package main

import (
	"os"
	"path/filepath"
	platformtarget "sec/internal/platform/target"
	"strings"
	"testing"

	"sec/internal/sema"
)

// Metadata is an independent target-cache dependency even when prepared source,
// manifest text, selected target and epoch are byte-for-byte unchanged.
// Exercise the actual cache with resolved snapshots, including source facts
// which have no manifest spelling and absent versus present-false values.
// Rules: rules/compiler/compiler_known_members.md — §113B(4);
// rules/projects/projects.md — §§33–34;
// rules/compiler/incremental_compilation.md — §§4–8.
func TestTargetAnalysisBuildInfoIdentity(t *testing.T) {
	base := sema.BuildInfoSnapshot{ProjectName: "Example", TargetName: "app", VariantName: "linux", VersionBuild: 7, OperatingSystem: "linux", Architecture: "arm64", BuildProfile: "release", TargetProfile: "hosted"}
	revisionA, revisionB, clean, dirty := "rev-a", "rev-b", false, true
	withRevision := base
	withRevision.SourceRevision = &revisionA
	otherRevision := base
	otherRevision.SourceRevision = &revisionB
	withClean := base
	withClean.DirtySourceStatus = &clean
	withDirty := base
	withDirty.DirtySourceStatus = &dirty
	nextBuild := base
	nextBuild.VersionBuild = 8
	var cache targetSnapshotCache
	computations := 0
	inputs := []any{"same source", "same manifest", "same target", uint64(5)}
	for _, test := range []struct {
		name     string
		snapshot *sema.BuildInfoSnapshot
		reuse    bool
	}{
		{"initial", &base, false},
		{"same-values-new-analyzer", &base, true},
		{"explicit-revision", &withRevision, false},
		{"changed-revision", &otherRevision, false},
		{"present-false", &withClean, false},
		{"present-true", &withDirty, false},
		{"changed-build", &nextBuild, false},
		{"unavailable", nil, false},
		{"restored-compatible-snapshot", &base, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			analyzer := sema.NewAnalyzer()
			expected := "unavailable"
			if test.snapshot != nil {
				analyzer.SetBuildInfo(*test.snapshot)
				expected = test.snapshot.CacheIdentity()
			}
			key, err := targetAnalysisIdentity(analyzer, inputs)
			if err != nil {
				t.Fatal(err)
			}
			before := computations
			data, err := cache.load(key, func() ([]byte, error) { computations++; return []byte(expected), nil })
			if err != nil || string(data) != expected {
				t.Fatalf("cached metadata = %q (%v), want %q", data, err, expected)
			}
			want := before + 1
			if test.reuse {
				want = before
			}
			if computations != want {
				t.Fatalf("computations = %d, want %d", computations, want)
			}
		})
	}
	analyzer := sema.NewAnalyzer()
	analyzer.SetBuildInfo(base)
	key, err := targetAnalysisIdentity(analyzer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := targetAnalysisIdentity(analyzer, []any{"different source", "same manifest", "same target", uint64(5)})
	if err != nil || changed == key {
		t.Fatal("adding metadata erased existing source dependencies")
	}
	if key, err := targetAnalysisIdentity(analyzer, []any{make(chan int)}); err == nil || key != "" {
		t.Fatal("unserializable inputs gained a reusable key")
	}
}

// Both the cache producer and its result must observe the selected metadata:
// changing only the stored build number changes a CTE-derived type contract.
// Rules: rules/compiler/compiler_known_members.md — §113B(2–4);
// rules/tooling/lsp.md — Target-aware analysis, Multi-target diagnostics.
func TestTargetAnalysisBuildInfoContract(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cmd", "app", "target.sec")
	manifestPath := filepath.Join(root, ".sec", "sec.toml")
	manifest, err := os.ReadFile("../../testdata/platform/buildinfo/source_facts.toml")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/lsp/build_info_cache/target.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Dir(path), filepath.Dir(manifestPath)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(manifestPath, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(string(manifest))
	target := platformtarget.Target{OS: "linux", Arch: "arm64"}
	overlay := sourceOverlay{Sources: map[string]string{normalizedSourcePath(path): string(source)}}
	run := func() targetAnalysisSnapshot {
		t.Helper()
		snapshot, err := analysisForTarget(uriFromPath(path), string(source), overlay, target)
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	first := run()
	if len(first.Errors) != 1 {
		t.Fatalf("build 7 should reject value 8: %v", first.Errors)
	}
	sharedTargetAnalyses.mu.Lock()
	before := sharedTargetAnalyses.computations
	sharedTargetAnalyses.mu.Unlock()
	if warm := run(); warm.key != first.key {
		t.Fatal("equivalent analysis changed cache key")
	}
	sharedTargetAnalyses.mu.Lock()
	after := sharedTargetAnalyses.computations
	sharedTargetAnalyses.mu.Unlock()
	if before != after {
		t.Fatal("target cache did not reuse complete metadata")
	}
	write(strings.Replace(string(manifest), "build = 7", "build = 8", 1))
	second := run()
	if len(second.Errors) != 0 || second.key == first.key {
		t.Fatalf("build 8 reused stale contract result: %v", second.Errors)
	}
}
