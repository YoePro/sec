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

// LSP operation hover consumes the same canonical failed query as CLI and never
// substitutes host layout information for an unverified selected target.
// Rules: MD-141 correction §10(1); rules/tooling/lsp.md — "Hover".
func TestLayoutFailureContextLSP(t *testing.T) {
	file := "../../testdata/sema/layout_failure_context.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "baremetal", TargetArch: "cortex-m3", Profile: "freestanding", PointerWidthBits: 32, Endianness: layout.LittleEndian})
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	found := false
	for _, fact := range a.AllocationContextFacts() {
		if fact.LayoutFailure == "" {
			continue
		}
		found = true
		hover := allocationOperationHover(a, fact.Source)
		if !strings.Contains(hover, fact.LayoutFailure) {
			t.Fatalf("hover lost canonical reason: %s", hover)
		}
	}
	if !found {
		t.Fatal("missing selected-target layout failure")
	}
}
