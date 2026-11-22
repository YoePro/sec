package main

import (
	"bytes"
	"os"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestThreadCurrentTargetJSON transports selected-plan physical-thread failure
// without reporting a runtime error or a compiler implementation limitation.
// Rules: rules/concurrency/cancellation.md §24; rules/tooling/diagnostics.md §§14–15.
func TestThreadCurrentTargetJSON(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/physical_thread/current.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "current.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	base := layout.ResolvedScalarPlan{TargetOS: "test", TargetArch: "test", Profile: "no-physical-thread", PointerWidthBits: 64, Endianness: layout.LittleEndian}
	plan, err := base.WithPhysicalThreadCapability(layout.PhysicalThreadCapability{Availability: layout.PhysicalThreadUnsupported, Provenance: "selected profile contract", Reason: "physical identity unavailable"})
	if err != nil {
		t.Fatal(err)
	}
	errs := sema.NewAnalyzerWithScalarPlan(plan).Analyze(program)
	if len(errs) != 1 {
		t.Fatal(errs)
	}
	var output bytes.Buffer
	reporter := &diagnosticReporter{format: diagnosticFormatJSON, output: &output}
	reporter.semaDiagnostic(errs[0], errs[0].Message)
	reporter.finish()
	doc := decodeOccurrenceDocument(t, output.String())
	if doc.Summary.Errors != 1 || len(doc.Occurrences) != 1 {
		t.Fatal(doc)
	}
	d := doc.Occurrences[0]
	if d.ID == nil || *d.ID != diagnostics.PhysicalThreadCapability || d.Unregistered || d.Primary == nil || d.Primary.Span.Start.Line != 4 || len(d.Help) == 0 || !strings.Contains(d.Message.Text, "no-physical-thread") {
		t.Fatal(d)
	}
}
