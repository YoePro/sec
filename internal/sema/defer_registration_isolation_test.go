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

// TestDeferRegistrationIsolation verifies that Reset/Release are validated as
// future cleanup without advancing the continuation's epoch or consuming its
// Arena. A rejected cleanup body must restore state too. Immediate invalidation
// remains effective, and each mandatory analysis depth uses the same policy.
// Rules: rules/control-flow/defer.md §§2–3,9,29;
// rules/memory/arena.md §§35–38,43–44.
func TestDeferRegistrationIsolation(t *testing.T) {
	for _, name := range []string{"valid", "immediate_invalid", "body_invalid", "release_body_invalid"} {
		source, err := os.ReadFile("../../testdata/control_flow/defer_arena_continuation/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", name, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(source), name+".sec"))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errs := a.Analyze(program)
					want := map[string]int{"valid": 0, "immediate_invalid": 3, "body_invalid": 1, "release_body_invalid": 1}[name]
					if len(errs) != want {
						t.Fatalf("diagnostics: %+v", errs)
					}
					if name == "release_body_invalid" {
						if errs[0].Line != 8 || !strings.Contains(errs[0].Message, "cannot use values after arena mem was reset") {
							t.Fatal(errs)
						}
						if len(a.resolvedDeferPlans) != 0 {
							t.Fatal("rejected cleanup published an executable plan")
						}
					}
				})
			}
		}
	}
}
