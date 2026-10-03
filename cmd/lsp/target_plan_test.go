package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLSPAnalyzerUsesProjectScalarPlan(t *testing.T) {
	dir := t.TempDir()
	manifestDir := filepath.Join(dir, ".sec")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `[project]
name = "target-plan"

[variant.embedded]
os = "linux"
arch = "armv7"

[target.app]
kind = "command"
source = "cmd/app"
variants = [
    "embedded",
]
`
	if err := os.WriteFile(filepath.Join(manifestDir, "sec.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(dir, "cmd", "app", "main.sec")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("module main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := newLSPAnalyzer(uriFromPath(sourcePath))
	if got := analyzer.Types()["int"].BitWidth; got != 32 {
		t.Fatalf("LSP int width = %d, want project-selected 32", got)
	}
	if got := analyzer.Types()["uint"].BitWidth; got != 32 {
		t.Fatalf("LSP uint width = %d, want project-selected 32", got)
	}
	if got := analyzer.Types()["float"].FloatBits; got != 32 {
		t.Fatalf("LSP float width = %d, want project-selected 32", got)
	}
	reported := analyze(uriFromPath(sourcePath), "module main\n\nfn Value() void {\n    let value: int := 3000000000\n}\n")
	assertDiagnosticMessage(t, reported, "overflows int")
}

func TestLSPScalarPlanRejectsAmbiguousProjectVariants(t *testing.T) {
	dir := t.TempDir()
	manifestDir := filepath.Join(dir, ".sec")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := `[variant.linux32]
os = "linux"
arch = "armv7"

[variant.linux64]
os = "linux"
arch = "amd64"

[target.app]
source = "."
variants = ["linux32", "linux64"]
`
	if err := os.WriteFile(filepath.Join(manifestDir, "sec.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(dir, "main.sec")
	if _, err := lspScalarPlan(sourcePath); err == nil {
		t.Fatal("lspScalarPlan accepted a project with two distinct active scalar plans")
	}
	if got := newLSPAnalyzer(uriFromPath(sourcePath)).Types()["int"].BitWidth; got != 0 {
		t.Fatalf("ambiguous target unexpectedly selected %d-bit int", got)
	}
}
