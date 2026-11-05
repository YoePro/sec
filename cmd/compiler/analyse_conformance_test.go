package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema"
	"strings"
	"testing"
)

// The real CLI consumes target-specific Sema conformance facts, exposes absent
// declarations, preserves failure IDs/locations and never labels an unavailable
// proof valid. Explicit target selection and family isolation stay effective.
// Rules: rules/tooling/lsp.md — Interface conformance code lens;
// rules/tooling/cli.md §§5–6; rules/declarations/interfaces.md §§4–6,12.
func TestAnalyseCrossTargetConformance(t *testing.T) {
	const fixture = "../../testdata/tooling/analyse_conformance/storage"
	decode := func(args ...string) (analysisReportDocument, string, int) {
		t.Helper()
		stdout, stderr, code := runCLIForDiagnostics(t, append([]string{"analyse", "--json"}, args...)...)
		var doc analysisReportDocument
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
			t.Fatalf("%v: %s %s", err, stdout, stderr)
		}
		return doc, stderr, code
	}
	doc, stderr, code := decode(fixture)
	if code != 0 || stderr != "" || len(doc.Conformances) != 4 {
		t.Fatalf("matrix: %d %s %+v", code, stderr, doc)
	}
	states := map[string]string{}
	for _, entry := range doc.Conformances {
		states[entry.Target+":"+entry.Type+":"+entry.Interface] = entry.State
	}
	for _, key := range []string{"linux-amd64:Handle:Native", "windows-amd64:Handle:Native", "linux-amd64:Handle:Writable"} {
		if states[key] != "Valid" {
			t.Fatalf("%s: %v", key, states)
		}
	}
	if states["windows-amd64:Handle:Writable"] != "missing" {
		t.Fatal(states)
	}
	if doc.Summary.Unproven != 1 {
		t.Fatal(doc.Summary)
	}
	again, _, _ := decode(fixture)
	left, _ := json.Marshal(doc.Conformances)
	right, _ := json.Marshal(again.Conformances)
	if string(left) != string(right) {
		t.Fatal("nondeterministic conformance report")
	}
	selected, stderr, code := decode("--target", "linux-amd64", fixture)
	if code != 0 || stderr != "" || len(selected.Conformances) != 2 || selected.Summary.Unproven != 0 {
		t.Fatalf("selected: %d %s %+v", code, stderr, selected)
	}
	only, _, code := decode("closures", fixture)
	if code != 0 || len(only.Conformances) != 0 {
		t.Fatalf("family selection: %d %+v", code, only)
	}
	verbose, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", fixture)
	if code != 0 || stderr != "" || !strings.Contains(verbose, "interface conformance:") || !strings.Contains(verbose, "windows-amd64: storage.Handle implements Writable: missing") {
		t.Fatalf("verbose: %d %s %s", code, verbose, stderr)
	}

	single, stderr, code := decode(filepath.Join(fixture, "native.sec"))
	if code != 0 || stderr != "" || len(single.Conformances) != 4 {
		t.Fatalf("single-file matrix: %d %s %+v", code, stderr, single)
	}
	// Configured Variants contribute the same deduplicated physical targets
	// as lenses, even when no source file names that architecture.
	projectRoot := t.TempDir()
	projectDir := filepath.Join(projectRoot, "storage")
	for _, directory := range []string{projectDir, filepath.Join(projectRoot, ".sec")} {
		if err := os.Mkdir(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"native.sec", "native.linux.sec", "native.windows.sec"} {
		data, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := os.ReadFile("../../testdata/tooling/analyse_conformance/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".sec", "sec.toml"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	configured, stderr, code := decode(projectDir)
	if code != 0 || stderr != "" || len(configured.Conformances) != 6 {
		t.Fatalf("project matrix: %d %s %+v", code, stderr, configured)
	}
	small := 0
	for _, entry := range configured.Conformances {
		if entry.Target == "linux-armv7" {
			small++
			if entry.State != "missing" {
				t.Fatal(entry)
			}
		}
	}
	if small != 2 {
		t.Fatal("configured 32-bit variant absent or duplicated", configured.Conformances)
	}

	clean, stderr, code := runCLIForDiagnostics(t, "analyse", "--target", "linux-amd64", fixture)
	if code != 0 || clean != "" || stderr != "" {
		t.Fatalf("clean human report: %d %q %q", code, clean, stderr)
	}
	directed, stderr, code := decode(filepath.Join(fixture, "native.linux.sec"))
	if code != 0 || stderr != "" || len(directed.Conformances) != 2 {
		t.Fatalf("directed file: %d %s %+v", code, stderr, directed)
	}
	// Make the Windows implementation invalid without modifying repository fixtures.
	dir := filepath.Join(t.TempDir(), "storage")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"native.sec", "native.linux.sec", "native.windows.sec"} {
		data, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "native.windows.sec" {
			data = []byte(strings.Replace(string(data), "Size()", "Size(extra: int)", 1))
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	invalid, stderr, code := decode(dir)
	if code != 3 || stderr != "" || invalid.Status != "rejected" || len(invalid.Diagnostics) == 0 {
		t.Fatalf("failure: %d %s %+v", code, stderr, invalid)
	}
	found := false
	for _, entry := range invalid.Conformances {
		if entry.Target == "windows-amd64" && entry.Interface == "Native" {
			if len(entry.Failures) != 1 || entry.Failures[0].Member != "Size" || entry.Failures[0].ID == "" || !strings.HasSuffix(entry.Failures[0].Location.File, "native.windows.sec") {
				t.Fatal(entry)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing failed conformance")
	}
}

// A declaration without any explicit implementation must never be presented
// as a successful empty target set.
// Rules: rules/tooling/lsp.md — Interface conformance code lens, Unknown facts.
func TestAnalyseConformanceWithoutImplementers(t *testing.T) {
	stdout, stderr, code := runCLIForDiagnostics(t, "analyse", "--json", "../../testdata/tooling/analyse_conformance/empty/empty.sec")
	var doc analysisReportDocument
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil || code != 0 || stderr != "" || len(doc.Conformances) != 1 || doc.Conformances[0].State != "missing" || doc.Summary.Unproven != 1 {
		t.Fatalf("%v: %d %s %s", err, code, stdout, stderr)
	}
}

// Equal type spellings in distinct modules cannot cover each other's missing
// conformance. Unknown Sema facts stay unknown in the projected matrix.
// Rules: rules/compiler/semantic_ir.md §109; rules/tooling/lsp.md — Unknown facts.
func TestAnalyseConformanceModuleIdentity(t *testing.T) {
	declaration := lexer.Token{File: "interface.sec", Line: 1, Column: 11, Lexeme: "Native"}
	fact := sema.InterfaceConformanceFact{TypeName: "Handle", TypeModule: "alpha", Interface: "Native", InterfaceDeclaration: declaration, TypeDeclaration: lexer.Token{File: "type.sec", Line: 1, Column: 6}, State: diagnostics.ProofValid}
	other := fact
	other.TypeModule = "beta"
	other.State = diagnostics.ProofUnproven
	report := analyseReport{counts: map[analyseReportClass]int{}}
	report.addConformances([]analyseConformanceTarget{
		{target: CompilerTarget{OS: "linux", Arch: "amd64"}, facts: []sema.InterfaceConformanceFact{fact}, interfaces: []lexer.Token{declaration}},
		{target: CompilerTarget{OS: "windows", Arch: "amd64"}, facts: []sema.InterfaceConformanceFact{other}, interfaces: []lexer.Token{declaration}},
	})
	if len(report.conformances) != 4 || report.counts[analyseClassUnproven] != 3 {
		t.Fatal(report.conformances, report.counts)
	}
	for _, entry := range report.conformances {
		if entry.Module == "beta" && entry.Target == "windows-amd64" && entry.State != "Unproven" {
			t.Fatal(entry)
		}
		if entry.Module == "alpha" && entry.Target == "windows-amd64" && entry.State != "missing" {
			t.Fatal(entry)
		}
	}
}
