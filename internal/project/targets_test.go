package project

import "testing"

// Target kind and source are read from `[target.<name>]` tables, nested
// version tables are skipped, and a source file maps to the Target whose
// source is its module directory or the file itself.
//
// Rules:
//   - rules/projects/projects.md — § 17 "Targets"
func TestManifestTargetsAndSourceLookup(t *testing.T) {
	targets := ParseManifestTargets(`[project]
name = "suite"

[target.sec]
kind = "command" # the compiler
source = "cmd/sec"

[target.sec.version]
build = 4

[target.board]
kind = 'firmware'
source = "main.sec"

[target.core]
kind = "library"
source = "internal/core"
`)
	if len(targets) != 3 || targets[2].Name != "sec" || targets[2].Kind != "command" || targets[2].Source != "cmd/sec" || targets[0].Kind != "firmware" {
		t.Fatalf("targets = %+v", targets)
	}
	for path, want := range map[string]string{
		"/repo/cmd/sec/main.sec":       "sec",
		"/repo/cmd/sec/flags.sec":      "sec",
		"/repo/main.sec":               "board",
		"/repo/internal/core/core.sec": "core",
	} {
		target, ok := TargetForSource(targets, "/repo", path)
		if !ok || target.Name != want {
			t.Errorf("%s -> %+v, %v; want %s", path, target, ok, want)
		}
	}
	if _, ok := TargetForSource(targets, "/repo", "/repo/cmd/other/main.sec"); ok {
		t.Error("a file outside every Target source must not match")
	}
}
