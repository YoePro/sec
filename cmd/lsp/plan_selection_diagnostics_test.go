package main

import (
	"strings"
	"testing"
)

// MD-139: the editor reports the profile/allocation selection errors the CLI
// rejects before analysis, on the owning table header; a valid manifest
// reports none.
func TestPlanSelectionDiagnosticsOnManifest(t *testing.T) {
	manifest := `[project]
name = "demo"

[variant.linux-amd64]
os = "linux"
arch = "amd64"
profile = "baremetal"

[variant.linux-arm64]
os = "linux"
arch = "arm64"

[variant.linux-arm64.allocation]
mode = "sometimes"

[variant.windows-amd64]
os = "windows"
arch = "amd64"
profile = "hosted"
`
	var got []diagnostic
	for _, d := range planSelectionDiagnostics(manifest) {
		if d.Code == "manifest.plan-selection" {
			got = append(got, d)
		}
	}
	if len(got) != 2 {
		t.Fatalf("diagnostics = %+v", got)
	}
	lines := map[int]string{}
	for _, d := range got {
		lines[d.Range.Start.Line] = d.Message
	}
	if !strings.Contains(lines[3], "reclassify") || !strings.Contains(lines[12], "sometimes") {
		t.Fatalf("diagnostics by line = %v", lines)
	}
	if extra := planSelectionDiagnostics("[project]\nname = \"demo\"\n\n[variant.linux-amd64]\nos = \"linux\"\narch = \"amd64\"\nprofile = \"hosted\"\n"); len(extra) != 0 {
		t.Fatalf("valid manifest: %+v", extra)
	}
}
