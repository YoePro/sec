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

// TestMutexTargetCapabilityLSP preserves target/profile-specific Sema diagnostics
// and ranges through the editor's canonical transport.
// Rules: rules/concurrency/mutex.md §63; rules/tooling/lsp.md — Shared diagnostic model.
func TestMutexTargetCapabilityLSP(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/mutex_target/acquisitions.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "mutex-target.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	base := layout.ResolvedScalarPlan{TargetOS: "test", TargetArch: "test", Profile: "no-context-wait", PointerWidthBits: 32, Endianness: layout.LittleEndian}
	plan, err := base.WithMutexCapabilities([]layout.MutexCapability{{Acquisition: layout.MutexContext, Availability: layout.MutexUnsupported, Provenance: "test contract", Reason: "no conforming context wait"}})
	if err != nil {
		t.Fatal(err)
	}
	errs := sema.NewAnalyzerWithScalarPlan(plan).Analyze(program)
	if len(errs) != 1 {
		t.Fatal(errs)
	}
	d := semaDiagnostic(errs[0], 1, string(source))
	if d.Code != diagnostics.MutexTargetCapability || d.Severity != 1 || d.Range.Start.Line != 5 || d.Range.End.Character <= d.Range.Start.Character || !strings.Contains(d.Message, "no-context-wait") {
		t.Fatal(d)
	}
}
