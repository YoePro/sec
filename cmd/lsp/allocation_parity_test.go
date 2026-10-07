package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	lspserver "sec/internal/lsp/server"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/sema"
)

type allocationCLIOccurrence struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Primary  struct {
		Span struct {
			File  string
			Start struct{ Line, Column int }
		}
	} `json:"primary"`
	Message struct{ Text string }   `json:"message"`
	Notes   []struct{ Text string } `json:"notes"`
	Related []struct {
		Span struct {
			File  string
			Start struct{ Line, Column int }
		}
	} `json:"related"`
}

// TestAllocationConsumerParity compares actual compiler/analyse CLI executions
// with active LSP diagnostics and hover, using the same source and target.
// Depth budgets must not change mandatory allocation errors or canonical facts.
// Rules: rules/memory/allocation.md — §§24(4),(6),28(4),29(1)-(4);
// rules/tooling/lsp.md — "Target-aware analysis", "Shared diagnostic model".
func TestAllocationConsumerParity(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "sec")
	build := exec.Command("go", "build", "-o", binary, "../compiler")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	for _, target := range []string{"linux-amd64", "baremetal-cortex-m3"} {
		for _, fixture := range []string{"allocation_cause_paths_invalid.sec", "allocation_context_facts_valid.sec", "allocation_facts_valid.sec", "allocation_recursive_fixed_point_valid.sec", "allocation_recursive_fixed_point_invalid.sec", "allocation_physical_storage_valid.sec"} {
			t.Run(target+"/"+fixture, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join("../../testdata/sema", fixture))
				if err != nil {
					t.Fatal(err)
				}
				parts := strings.SplitN(target, "-", 2)
				source := fmt.Sprintf("#target(os: %q, arch: %q)\n", parts[0], parts[1]) + string(data)
				dir := t.TempDir()
				if err := os.Mkdir(filepath.Join(dir, "app"), 0755); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, "app", "main.sec")
				if err := os.WriteFile(path, []byte(source), 0644); err != nil {
					t.Fatal(err)
				}
				// Deliberately conflicting manifest catches accidental manifest precedence.
				if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
					t.Fatal(err)
				}
				manifest := "[variant.active]\nos = \"linux\"\narch = \"amd64\"\n[target.app]\nsource = \"app\"\nvariants = [\"active\"]\n"
				if target == "linux-amd64" {
					manifest = strings.ReplaceAll(strings.ReplaceAll(manifest, "linux", "baremetal"), "amd64", "cortex-m3")
				}
				if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte(manifest), 0644); err != nil {
					t.Fatal(err)
				}
				uri := uriFromPath(path)
				var lspReports [][]diagnostic
				for _, depth := range []string{"interactive", "standard", "deep"} {
					configured := manifest + "[analysis]\nlsp_depth = " + fmt.Sprintf("%q", depth) + "\n"
					if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), []byte(configured), 0644); err != nil {
						t.Fatal(err)
					}
					lspReports = append(lspReports, analyze(uri, source), analyzeDiagnosticBatch([]lspserver.Snapshot{{URI: uri, Text: source}}, nil)[uri])
				}
				for _, command := range []string{"emit-ir", "analyse"} {
					args := []string{command, path, "--target", target, "--diagnostic-format=json"}
					if command == "emit-ir" {
						args = append(args, "-o", "-")
					}
					cmd := exec.Command(binary, args...)
					var stdout, stderr bytes.Buffer
					cmd.Stdout = &stdout
					cmd.Stderr = &stderr
					runErr := cmd.Run()
					var document struct {
						Occurrences []allocationCLIOccurrence `json:"occurrences"`
					}
					if err := json.Unmarshal(stderr.Bytes(), &document); err != nil {
						t.Fatalf("%s: %v (%v)\n%s\n%s", command, err, runErr, stderr.String(), stdout.String())
					}
					for _, reported := range lspReports {
						allocation := map[string]diagnostic{}
						for _, diagnostic := range reported {
							if diagnostic.Code == "S1108" || diagnostic.Code == "S1098" {
								key := allocationParityKey(diagnostic.Code, diagnostic.Range.Start.Line+1, diagnostic.Range.Start.Character+1)
								if _, exists := allocation[key]; exists {
									t.Fatalf("duplicate owning occurrence: %s", key)
								}
								allocation[key] = diagnostic
							}
						}
						expected := 0
						if fixture == "allocation_cause_paths_invalid.sec" {
							expected = 4
						}
						if fixture == "allocation_recursive_fixed_point_invalid.sec" {
							expected = 2
						}
						if fixture == "allocation_context_facts_valid.sec" && target == "baremetal-cortex-m3" {
							expected = 2
						}
						if len(allocation) != expected {
							t.Fatalf("allocation error count %d; want %d", len(allocation), expected)
						}
						for _, occurrence := range document.Occurrences {
							if occurrence.ID != "S1108" && occurrence.ID != "S1098" {
								continue
							}
							point := occurrence.Primary.Span.Start
							key := allocationParityKey(occurrence.ID, point.Line, point.Column)
							diagnostic, found := allocation[key]
							if !found || diagnostic.Severity != 1 || occurrence.Severity != "error" || !strings.Contains(diagnostic.Message, occurrence.Message.Text) {
								t.Fatalf("%s mismatch: %+v versus %+v", command, occurrence, reported)
							}
							for _, note := range occurrence.Notes {
								if !strings.Contains(diagnostic.Message, strings.Split(note.Text, " at ")[0]) {
									t.Fatalf("lost cause: %s\n%s", note.Text, diagnostic.Message)
								}
							}
							for _, related := range occurrence.Related {
								found := false
								for _, link := range diagnostic.RelatedInformation {
									if link.Location.URI == uriFromPath(related.Span.File) && link.Location.Range.Start.Line+1 == related.Span.Start.Line && link.Location.Range.Start.Character+1 == related.Span.Start.Column {
										found = true
									}
								}
								if !found {
									t.Fatalf("lost related location: %+v in %+v", related, diagnostic)
								}
							}
							delete(allocation, key)
						}
						if len(allocation) != 0 {
							t.Fatalf("%s omitted %d allocation diagnostics (%v):\n%s\n%s", command, len(allocation), runErr, stderr.String(), stdout.String())
						}
					}
					// Mandatory errors stop CLI report generation; LSP retains partial facts.
					if command == "analyse" && runErr == nil {
						analyzer := analyzeNavigationSource(uri, source)
						graph := analyzer.CallGraph()
						checked := 0
						for _, node := range graph.Nodes() {
							if filepath.Clean(node.Declaration.File) != path {
								continue
							}
							fact := analyzer.AllocationFact(node.ID)
							if !strings.Contains(stdout.String(), node.Name+": allocation "+string(fact.Knowledge)) {
								t.Fatalf("analyse lost fact: %+v (%v)\n%s\n%s", fact, runErr, stdout.String(), stderr.String())
							}
							// Declaration hover currently supports top-level functions.
							if node.Kind != sema.CallableBodyNamedFunction || node.ImplTarget != "" {
								continue
							}
							hover, ok := hoverForSource(uri, source, diagnosticTokenStart(source, node.Declaration))
							if !ok || !strings.Contains(hover.Contents.Value, "Allocation behavior: `"+string(fact.Knowledge)+"`") {
								t.Fatalf("hover lost fact: %s %+v", node.Name, hover)
							}
							if fact.HasUnknown && (!strings.Contains(stdout.String(), "unresolved allocation behavior") || !strings.Contains(hover.Contents.Value, "Unresolved allocation behavior: `yes`")) {
								t.Fatalf("lost unknown behavior: %s", node.Name)
							}
							for _, evidence := range fact.UnknownEvidence {
								if !strings.Contains(stdout.String(), evidence.Reason) || !strings.Contains(hover.Contents.Value, evidence.Reason) {
									t.Fatalf("lost evidence: %s", evidence.Reason)
								}
							}
							for _, cause := range [][]sema.CallableID{fact.AllocationPath, fact.UnknownPath} {
								if len(cause) == 0 {
									continue
								}
								var names []string
								for _, id := range cause {
									step, _ := graph.Node(id)
									names = append(names, step.Name)
								}
								if !strings.Contains(stdout.String(), strings.Join(names, " -> ")) || !strings.Contains(hover.Contents.Value, "`"+strings.Join(names, "` -> `")+"`") {
									t.Fatalf("lost path: %v", names)
								}
							}
							for _, context := range analyzer.AllocationContexts(node.ID) {
								want := sema.AllocationContextDescription(context)
								if !strings.Contains(stdout.String(), want) || !strings.Contains(hover.Contents.Value, want) {
									t.Fatalf("context mismatch: %s\n%s\n%s", want, stdout.String(), hover.Contents.Value)
								}
							}
							checked++
						}
						if checked == 0 {
							t.Fatal("no source facts compared")
						}
					}
				}
			})
		}
	}
}

// allocationParityKey identifies one owning ASCII fixture occurrence in the
// compiler's 1-based coordinates, independently of diagnostic ordering.
// Rules: rules/tooling/diagnostics.md — §§9,28.
func allocationParityKey(id string, line, column int) string {
	return fmt.Sprintf("%s:%d:%d", id, line, column)
}

// TestAllocationTargetOverlay selects the unsaved directive and excludes
// directives from assembled dependencies when resolving the active profile.
// Rules: rules/memory/allocation.md — §29(1),(5);
// rules/tooling/lsp.md — "Document synchronization", "Target-aware analysis".
func TestAllocationTargetOverlay(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/allocation_context_facts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "main.sec")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(path)
	for _, target := range []struct{ os, arch, profile string }{{"linux", "amd64", "hosted"}, {"baremetal", "cortex-m3", "freestanding"}, {"linux", "amd64", "hosted"}} {
		source := fmt.Sprintf("#target(os: %q, arch: %q)\n%s", target.os, target.arch, data)
		reported := analyze(uri, source)
		unavailable := false
		for _, item := range reported {
			unavailable = unavailable || item.Code == "S1098"
		}
		if unavailable != (target.profile == "freestanding") {
			t.Fatalf("%s: %+v", target.profile, reported)
		}
		offset := strings.Index(source, "fn Forward(") + 3
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, "profile "+target.profile) {
			t.Fatal(target, hover)
		}
	}
}

// TestAllocationTargetIgnoresDependencyDirective keeps imported target metadata
// from replacing the active source's target-independent fallback.
// Rules: rules/memory/allocation.md — §29(1); rules/tooling/lsp.md — "Target-aware analysis".
func TestAllocationTargetIgnoresDependencyDirective(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/allocation_context_facts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	program := parseProgramForLSP(uri, string(data))
	imported := parseProgramForLSP(uriFromPath(filepath.Join(t.TempDir(), "other.sec")), fmt.Sprintf("#target(os: %q, arch: %q)\n%s", "baremetal", "cortex-m3", data))
	if program == nil || imported == nil {
		t.Fatal("fixture parse failed")
	}
	directive, ok := imported.Statements[0].(*ast.TargetDirective)
	if !ok {
		t.Fatal("missing imported directive")
	}
	program.Statements = append(program.Statements, directive)
	if width := newLSPAnalyzer(uri, program).Types()["int"].BitWidth; width != 0 {
		t.Fatalf("import selected %d-bit target", width)
	}
}
