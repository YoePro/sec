package sema

import (
	"fmt"
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestSchedulingThreadConfiguration locks the shared scheduling/threads source
// contract at every analysis depth and scalar width. Preferred/Required remain
// explicit, CpuSet is nominal, priorities are closed portable intents, and no
// runtime mutation API is synthesized. Source acceptance is not target support.
// Rules: rules/concurrency/scheduling.md — §§32–36;
// rules/concurrency/threads.md — §§4,6–7,10–11.
func TestSchedulingThreadConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		errors int
	}{{"valid", 0}, {"invalid", 8}} {
		file := "../../testdata/sema/scheduling_thread_config_" + tc.name + ".sec"
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
				t.Run(fmt.Sprintf("%s/%d/%s", tc.name, width, depth), func(t *testing.T) {
					p := parser.New(lexer.NewWithFile(string(data), file))
					program := p.ParseProgram()
					if len(p.Errors()) != 0 {
						t.Fatal(p.Errors())
					}
					a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
					errors := a.Analyze(program)
					if len(errors) != tc.errors {
						t.Fatalf("want %d errors, got %d: %+v", tc.errors, len(errors), errors)
					}
					if tc.name == "invalid" {
						// One diagnostic at each independent forbidden operation;
						// cascades elsewhere cannot mask an accepted bad form.
						lines := map[int]bool{4: false, 7: false, 10: false, 13: false, 16: false, 19: false, 22: false, 25: false}
						for _, e := range errors {
							seen, expected := lines[e.Line]
							if !expected || seen || e.File != file {
								t.Fatalf("misplaced or duplicate diagnostic: %+v", e)
							}
							lines[e.Line] = true
						}
					}
				})
			}
		}
	}
}
