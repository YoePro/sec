package main

import (
	"os"
	"path/filepath"
	"testing"

	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

// MD-143 § 113B(3)-(4): the editor resolves the same snapshot as the CLI
// from the unsaved manifest overlay, using the stored build number; a
// file without a resolvable plan gets no snapshot.
func TestLSPBuildInfoUsesOverlayManifest(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "cmd", "worker", "main.sec")
	if err := os.MkdirAll(filepath.Dir(source), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("module main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := "[project]\nname='Worker Project'\nuuid='550e8400-e29b-41d4-a716-446655440000'\n[version]\nmajor=2\nminor=0\nrevision=1\nbuild=7\nformat='[major].[minor].[revision]+[build]'\n[build]\nprofile='release'\n[target.worker]\nkind='command'\nsource='cmd/worker'\nvariants=['linux']\n[variant.linux]\nos='linux'\narch='amd64'\n"
	overlay := sourceOverlay{Sources: map[string]string{filepath.Join(root, ".sec", "sec.toml"): manifest}}
	definition, _ := platformtarget.Find(platformtarget.Target{OS: "linux", Arch: "amd64"})
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	analyzer := sema.NewAnalyzerWithScalarPlanAndDepth(plan, sema.AnalysisInteractive)
	applyLSPBuildInfo(analyzer, source, overlay)
	snapshot, ok := analyzer.BuildInfo()
	if !ok || snapshot.TargetName != "worker" || snapshot.VariantName != "linux" || snapshot.VersionRendered != "2.0.1+7" || snapshot.BuildProfile != "release" {
		t.Fatalf("snapshot = %+v (%v)", snapshot, ok)
	}
	other := sema.NewAnalyzerWithScalarPlanAndDepth(plan, sema.AnalysisInteractive)
	applyLSPBuildInfo(other, filepath.Join(root, "tools", "x.sec"), overlay)
	if _, ok := other.BuildInfo(); ok {
		t.Fatal("source outside every Target received a snapshot")
	}
}
