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
	"sec/internal/platform/target"
)

// TestMainTaskIdentityIndependentOfTarget verifies that every registered target
// uses the same main/task separation at every analysis budget. A target's
// physical execution machinery cannot grant main source-visible TaskContext.
// The suggested fallible-spawn repair is valid source on the same plans; this
// verifies frontend identity, not runtime creation/target capability support.
// Rules: rules/concurrency/scheduling.md — §22(1–5);
// rules/corrections/applied/scheduling-v2-cross-rulebook-correction-20260927.md — §§4–5;
// rules/concurrency/tasks.md — §4(1–7).
func TestMainTaskIdentityIndependentOfTarget(t *testing.T) {
	invalid, err := os.ReadFile("../../testdata/sema/main_task_context_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	guidance, err := os.ReadFile("../../testdata/sema/main_task_guidance_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	profiles := map[string]bool{}
	widths := map[uint16]bool{}
	for _, definition := range target.Definitions() {
		var plan layout.ResolvedScalarPlan
		if definition.PointerWidthBits == 0 {
			// Registry placeholders have no scalar realization yet. Preserve
			// that Unknown instead of borrowing the compiler host's width.
			plan = layout.ResolvedScalarPlan{TargetOS: definition.OS, TargetArch: definition.Arch, Profile: definition.Profile}
		} else {
			plan, err = definition.ScalarPlan()
			if err != nil {
				t.Fatal(err)
			}
		}
		profiles[plan.Profile], widths[plan.PointerWidthBits] = true, true
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("%s/%s/%s", definition.OS, definition.Arch, depth), func(t *testing.T) {
				for _, valid := range []bool{false, true} {
					source := invalid
					if valid {
						source = guidance
					}
					p := parser.New(lexer.NewWithFile(string(source), "main-task-target.sec"))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(plan, depth)
					errors := a.Analyze(program)
					want := 6
					if valid {
						want = 0
					}
					if len(errors) != want {
						t.Fatalf("valid=%v: %+v", valid, errors)
					}
					for _, e := range errors {
						if e.ID != diagnostics.LogicalTaskContextRequired || !strings.Contains(e.Message, "ordinary program entry is not a logical task") ||
							!strings.Contains(e.Help, "let work := try spawn Work() { Err(_) => { return } }") {
							t.Fatalf("unexpected target-dependent context/invalid repair: %+v", e)
						}
					}
				}
			})
		}
	}
	if len(profiles) < 2 || !widths[32] || !widths[64] {
		t.Fatalf("insufficient target coverage: profiles=%v widths=%v", profiles, widths)
	}
}
