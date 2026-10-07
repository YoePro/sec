package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestAllocationContextHover checks real callable hovers, all transitive sites,
// domain roles and absence of ambient context on folded and spawned execution.
// Rules: rules/memory/allocation.md — §§5,6,17,29(1),(3); rules/tooling/lsp.md — "Allocation and escape analysis".
func TestAllocationContextHover(t *testing.T) {
	file := "../../testdata/sema/allocation_context_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	cases := map[string][]string{
		"Render":      {"implicit allocation context: available; profile hosted; origin Arena; domain identity unresolved"},
		"Interpolate": {"implicit allocation context: available; profile hosted"},
		"Allocate":    {"explicit Arena allocation domain:", "(receiver mem)", "(receiver moved)"},
		"OtherDomain": {"explicit Arena allocation domain:", "(receiver mem)"},
		"Created":     {"created Arena domain:", "backing allocation context unresolved"},
		"Forward":     {"implicit allocation context:", "via `Forward` -> `Render`"},
		"Combined":    {"implicit allocation context:", "explicit Arena allocation domain:", "allocation context/domain unresolved"},
		"Recursive":   {"implicit allocation context:", "via `Recursive` -> `Render`"},
		"Deferred":    {"implicit allocation context:"},
		"Folded":      {}, "Spawner": {},
	}
	for name, wants := range cases {
		offset := strings.Index(source, "fn "+name+"(") + 3
		hover, ok := hoverForSource("file:///tmp/sec-allocation-contexts/main.sec", source, offsetPosition(source, offset))
		if !ok {
			t.Fatal("missing hover", name)
		}
		for _, want := range wants {
			if !strings.Contains(hover.Contents.Value, want) {
				t.Fatal(name, want, hover.Contents.Value)
			}
		}
		if len(wants) == 0 && (strings.Contains(hover.Contents.Value, "implicit allocation context:") || strings.Contains(hover.Contents.Value, "explicit Arena allocation domain:")) {
			t.Fatal("invented context", name, hover)
		}
	}
}

// TestAllocationContextHoverProfiles verifies that rendering consumes selected
// compiler facts even when a profile rejects the operation; it does not infer
// availability itself or replace semantic diagnostics.
// Rules: rules/memory/allocation.md — §§17(7)-(9),22(4),29(1),(3).
func TestAllocationContextHoverProfiles(t *testing.T) {
	file := "../../testdata/sema/allocation_context_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"hosted", "embedded-arena", "noalloc", "freestanding"} {
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "baremetal", TargetArch: "cortex-m3", Profile: profile, PointerWidthBits: 32})
		a.Analyze(program)
		graph := a.CallGraph()
		for _, node := range graph.Nodes() {
			if node.Name != "Forward" {
				continue
			}
			hover := strings.Join(allocationHoverLines(a, graph, node.ID), "\n")
			for _, fact := range a.AllocationContexts(node.ID) {
				if !strings.Contains(hover, sema.AllocationContextDescription(fact)) {
					t.Fatal(profile, fact, hover)
				}
			}
			if !strings.Contains(hover, "profile "+profile) {
				t.Fatal(profile, hover)
			}
		}
	}
}

// TestAllocationOperationHover checks member/operator positions without losing
// their original semantic hover, and rejects coincident locations in other files.
// Rules: rules/memory/allocation.md — §29(1),(3); rules/tooling/lsp.md — "Hover".
func TestAllocationOperationHover(t *testing.T) {
	file := "../../testdata/sema/allocation_context_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, tc := range []struct {
		marker string
		skip   int
		want   string
	}{
		{"mem.New[byte]", 4, "explicit Arena allocation domain:"},
		{"moved.Alloc[byte]", 6, "explicit Arena allocation domain:"},
		{"Arena.WithCapacity", 6, "created Arena domain:"},
		{"+ name", 0, "implicit allocation context: available; profile hosted"},
	} {
		offset := strings.Index(source, tc.marker) + tc.skip
		hover, ok := hoverForSource("file:///tmp/sec-allocation-operations/main.sec", source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, "**Allocation context**") || !strings.Contains(hover.Contents.Value, tc.want) {
			t.Fatal(tc, hover)
		}
	}
	p := parser.New(lexer.NewWithFile(source, file))
	a := sema.NewAnalyzer()
	a.Analyze(p.ParseProgram())
	fact := a.AllocationContextFacts()[0]
	token := fact.Source
	token.File = "different.sec"
	if allocationOperationHover(a, token) != "" {
		t.Fatal("operation matched by position without source identity")
	}
}
