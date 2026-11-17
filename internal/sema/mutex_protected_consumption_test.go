package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMutexProtectedOwnerConsumption verifies that forwarding cannot grant
// ownership of complete T: whole-self consumption is invalid at the method
// declaration, and the nominal guard is never an implicit or constructor cast
// to T. Guard moves and ordinary protected scalar copies remain valid.
// Rules: rules/concurrency/mutex.md — §§28,30(3–5),33(4),36(1–3);
// rules/declarations/functions.md — §22; rules/memory/ownership.md — §9.
func TestMutexProtectedOwnerConsumption(t *testing.T) {
	for _, valid := range []bool{true, false} {
		name := "mutex_protected_consumption_valid.sec"
		want := 0
		if !valid {
			name = "mutex_protected_consumption_invalid.sec"
			want = 6
		}
		source, err := os.ReadFile("../../testdata/sema/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", name, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(source), name))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errors := a.Analyze(program)
					if len(errors) != want {
						t.Fatalf("got %d errors, want %d: %+v", len(errors), want, errors)
					}
					if !valid {
						whole := 0
						for _, diagnostic := range errors {
							if strings.Contains(diagnostic.Message, "cannot consume complete self") {
								whole++
							}
						}
						if whole != 3 {
							t.Fatalf("whole-self failures=%d: %+v", whole, errors)
						}
					}
				})
			}
		}
	}
}
