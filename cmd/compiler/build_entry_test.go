package main

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/sema"
)

// sec build validates the entry of the Target that owns the input file: the
// manifest's Target kind when the file belongs to a declared Target, and a
// command entry otherwise.
//
// Rules:
//   - rules/compiler/initialization.md — § 20 "Target entry contracts"
//   - rules/projects/projects.md — § 17 "Targets"
func TestBuildTargetKindComesFromTheManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".sec"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "[project]\nname = \"board\"\n\n[target.board]\nkind = \"firmware\"\nsource = \"cmd/board\"\n"
	if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if kind := buildTargetKind(filepath.Join(root, "cmd", "board", "main.sec")); kind != sema.ProgramTargetFirmware {
		t.Errorf("declared firmware Target kind = %s", kind)
	}
	if kind := buildTargetKind(filepath.Join(root, "tools", "probe.sec")); kind != sema.ProgramTargetCommand {
		t.Errorf("file outside every Target kind = %s, want command", kind)
	}
	if kind := buildTargetKind(filepath.Join(t.TempDir(), "main.sec")); kind != sema.ProgramTargetCommand {
		t.Errorf("file without a manifest kind = %s, want command", kind)
	}
}

// Rules: rules/projects/projects.md — §§3.2–3.3,17,49(1–2).
func TestBuildTargetKindUsesResolvedProject(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".sec", "sec.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, fixture, 0644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		kind sema.ProgramTargetKind
	}{
		{"cmd/sec/main.sec", sema.ProgramTargetCommand},
		{"compiler/library/library.sec", sema.ProgramTargetLibrary},
	} {
		if got := buildTargetKind(filepath.Join(root, filepath.FromSlash(tc.path))); got != tc.kind {
			t.Fatal(tc.path, got, tc.kind)
		}
	}
}
