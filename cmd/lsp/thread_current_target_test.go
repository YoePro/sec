package main

import (
	"os"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestThreadCurrentTargetLSP publishes the canonical target/profile identity
// denial with its exact source range and a repair that cannot fabricate a task.
// Rules: rules/concurrency/cancellation.md §24; rules/tooling/lsp.md — Shared diagnostic model.
func TestThreadCurrentTargetLSP(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/physical_thread/current.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "current.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	base := layout.ResolvedScalarPlan{TargetOS: "test", TargetArch: "test", Profile: "no-physical-thread", PointerWidthBits: 32, Endianness: layout.LittleEndian}
	plan, err := base.WithPhysicalThreadCapability(layout.PhysicalThreadCapability{Availability: layout.PhysicalThreadUnsupported, Provenance: "selected profile contract", Reason: "physical identity unavailable"})
	if err != nil {
		t.Fatal(err)
	}
	errs := sema.NewAnalyzerWithScalarPlan(plan).Analyze(program)
	if len(errs) != 1 {
		t.Fatal(errs)
	}
	d := semaDiagnostic(errs[0], 1, string(source))
	if d.Code != diagnostics.PhysicalThreadCapability || d.Severity != 1 || d.Range.Start.Line != 3 || d.Range.End.Character <= d.Range.Start.Character || !strings.Contains(d.Message, "no-physical-thread") {
		t.Fatal(d)
	}
}
