package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestInterruptEntryCalls consumes resolved direct, indirect and merged target
// identities. Capability errors for unimplemented entry attributes remain
// separate; helpers, overloads and unknown callbacks must not be misclassified.
// Rules: rules/platform/interrupts.md — §§2–4,25.
func TestInterruptEntryCalls(t *testing.T) {
	for _, fixture := range []struct {
		name string
		want int
	}{
		{"interrupt_entry_calls_invalid.sec", 7},
		{"interrupt_entry_helpers.sec", 0},
		{"interrupt_entry_testing_invalid_test.sec", 1},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + fixture.name)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", fixture.name, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(data), fixture.name))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errors := a.Analyze(program)
					count := 0
					for _, diagnostic := range errors {
						if diagnostic.ID == diagnostics.InterruptEntryNotCallable {
							count++
							if diagnostic.PreviousFile != fixture.name || diagnostic.PreviousLine == 0 || diagnostic.PreviousColumn == 0 || diagnostic.Line == 0 || diagnostic.Column == 0 || !strings.Contains(diagnostic.Help, "ordinary helper") {
								t.Fatalf("missing diagnostic provenance/help: %+v", diagnostic)
							}
						} else if diagnostic.ID != diagnostics.CompilerCapabilityUnavailable && diagnostic.ID != diagnostics.UnreachableStatement {
							t.Errorf("unexpected diagnostic: %+v", diagnostic)
						}
					}
					if count != fixture.want {
						t.Fatalf("entry-call diagnostics = %d, want %d: %v", count, fixture.want, errors)
					}
				})
			}
		}
	}
}
