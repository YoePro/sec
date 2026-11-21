package main

import (
	"bytes"
	"os"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"testing"
)

// TestMutexTargetCapabilityJSON transports real selected-plan Sema denials
// without confusing target support with compiler capability or runtime errors.
// Rules: rules/concurrency/mutex.md §§27,63; rules/tooling/diagnostics.md §§14–15.
func TestMutexTargetCapabilityJSON(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/mutex_target/acquisitions.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "mutex-target.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	base := layout.ResolvedScalarPlan{TargetOS: "test", TargetArch: "test", Profile: "no-locks", PointerWidthBits: 64, Endianness: layout.LittleEndian}
	var facts []layout.MutexCapability
	for _, mode := range []layout.MutexAcquisition{layout.MutexBlocking, layout.MutexTry, layout.MutexTimeout, layout.MutexContext} {
		facts = append(facts, layout.MutexCapability{Acquisition: mode, Availability: layout.MutexUnsupported, Provenance: "test contract", Reason: "no conforming lock"})
	}
	plan, err := base.WithMutexCapabilities(facts)
	if err != nil {
		t.Fatal(err)
	}
	errs := sema.NewAnalyzerWithScalarPlan(plan).Analyze(program)
	if len(errs) != 4 {
		t.Fatal(errs)
	}
	var output bytes.Buffer
	reporter := &diagnosticReporter{format: diagnosticFormatJSON, output: &output}
	for _, err := range errs {
		reporter.semaDiagnostic(err, err.Message)
	}
	reporter.finish()
	doc := decodeOccurrenceDocument(t, output.String())
	if doc.Summary.Errors != 4 {
		t.Fatal(doc)
	}
	for _, d := range doc.Occurrences {
		if d.ID == nil || *d.ID != diagnostics.MutexTargetCapability || d.Unregistered || d.Primary == nil || d.Primary.Span.Start.Line == 0 || len(d.Help) == 0 {
			t.Fatalf("incomplete target diagnostic: %+v", d)
		}
	}
}
