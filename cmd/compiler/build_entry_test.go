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
