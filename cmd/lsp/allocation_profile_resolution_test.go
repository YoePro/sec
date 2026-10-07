package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/layout"
	lspserver "sec/internal/lsp/server"
	platformtarget "sec/internal/platform/target"
)

// TestLSPAllocationCapabilities separates common allocation facts from scalar
// width ambiguity, rejecting mixed/invalid variants without a hosted guess.
// Source directives still override the manifest through the current AST.
// Rules: rules/memory/allocation.md — §§22,29(1),(5);
// rules/platform/target_profiles.md — §§3,31-32; rules/tooling/lsp.md — "Target-aware analysis".
func TestLSPAllocationCapabilities(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/allocation_context_facts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.sec")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(path)
	for _, test := range []struct {
		variants, profile string
		active            bool
	}{
		{`["host64", "host32"]`, "hosted", true},
		{`["host64", "bare"]`, "", false},
		{`["bare"]`, "freestanding", false},
		{`["absent"]`, "", false},
	} {
		manifest := "[variant.host64]\nos = \"linux\"\narch = \"amd64\"\n[variant.host32]\nos = \"linux\"\narch = \"armv7\"\n[variant.bare]\nos = \"baremetal\"\narch = \"cortex-m3\"\n[target.app]\nsource = \".\"\nvariants = " + test.variants + "\n"
		if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}
		want := layout.ResolveAllocationCapabilities(test.profile)
		if got := lspAllocationCapabilities(path); got != want {
			t.Fatal(test, got, want)
		}
		analyzer := newLSPAnalyzer(uri, parseProgramForLSP(uri, string(data)))
		if analyzer.AllocationCapabilities() != want {
			t.Fatal("analyzer ignored canonical facts", test, analyzer.AllocationCapabilities())
		}
		if strings.Contains(test.variants, ",") && analyzer.Types()["int"].BitWidth != 0 {
			t.Fatal("allocation resolution guessed scalar width")
		}
		diagnostics := analyze(uri, string(data))
		count := 0
		for _, item := range diagnostics {
			if item.Code == "S1098" {
				count++
			}
		}
		expected := 0
		if !test.active {
			expected = 2
		}
		if count != expected {
			t.Fatal(test, diagnostics)
		}
		batch := analyzeDiagnosticBatch([]lspserver.Snapshot{{URI: uri, Text: string(data)}}, nil)
		batchCount := 0
		for _, item := range batch[uri] {
			if item.Code == "S1098" {
				batchCount++
			}
		}
		if batchCount != expected {
			t.Fatal("batched capabilities differ", test, batch[uri])
		}
		offset := strings.Index(string(data), "fn Forward(") + 3
		hover, ok := hoverForSource(uri, string(data), offsetPosition(string(data), offset))
		profile := test.profile
		if profile == "" {
			profile = "unresolved"
		}
		if !ok || !strings.Contains(hover.Contents.Value, "profile "+profile) {
			t.Fatal(test, hover)
		}
	}
	if err := os.Remove(filepath.Join(dir, ".sec", "sec.toml")); err != nil {
		t.Fatal(err)
	}
	definition, found := platformtarget.Find(platformtarget.Host())
	if !found {
		t.Fatal("host is unregistered")
	}
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	if lspAllocationCapabilities(path) != plan.AllocationCapabilities() {
		t.Fatal("standalone source did not use registry host facts")
	}
	source := "#target(os: \"unknown\", arch: \"unknown\")\n" + string(data)
	if got := newLSPAnalyzer(uri, parseProgramForLSP(uri, source)).AllocationCapabilities(); got.HasActiveArenaContext() {
		t.Fatal("unknown explicit target acquired host capabilities", got)
	}
}
