package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// A recognized explicit requirement must never reach old callable semantics as
// an empty/inferred contract. The limit has a capability identity, not a source
// error. Reanalysis must retain that boundary without stale successful facts.
// Rules: rules/concurrency/cancellation.md — §24A(5)–(9);
// rules/compiler/compiler_pipeline.md — §32(2)–(4).
func TestExecutionRequirementCapabilityBoundary(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/execution_requirements.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "execution_requirements.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	for attempt := 0; attempt < 2; attempt++ {
		errors := a.Analyze(program)
		if len(errors) != 9 {
			t.Fatalf("%d errors, want 9 contracts: %v", len(errors), errors)
		}
		for _, err := range errors {
			if err.ID != diagnostics.CompilerCapabilityUnavailable || !strings.Contains(err.Error(), "explicit callable execution requirements") {
				t.Fatal(err)
			}
		}
	}
}
