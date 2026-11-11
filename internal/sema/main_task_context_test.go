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

// TestMainTaskContext rejects logical-task operations in ordinary main through
// direct, synchronous, function-value and deferred calls on either scalar plan.
// Rules: rules/concurrency/scheduling.md — §§21–22,50,57;
// rules/concurrency/cancellation.md — §§19–23.
func TestMainTaskContext(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, mode := range []string{"valid", "invalid"} {
			t.Run(fmt.Sprintf("%d/%s", width, mode), func(t *testing.T) {
				file := "../../testdata/sema/main_task_context_" + mode + ".sec"
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				p := parser.New(lexer.NewWithFile(string(data), file))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				errors := a.Analyze(program)
				if mode == "valid" {
					if len(errors) != 0 {
						t.Fatal(errors)
					}
					return
				}
				// Each invalid root call has its own diagnostic, even when
				// several paths reach the same context-requiring callable.
				wantLines := map[int]bool{16: true, 17: true, 18: true, 19: true, 21: true, 22: true}
				if len(errors) != len(wantLines) {
					t.Fatal(errors)
				}
				for _, d := range errors {
					if !wantLines[d.Line] || d.ID != diagnostics.LogicalTaskContextRequired || d.File != file || !strings.Contains(d.Message, "ordinary program entry is not a logical task") || !strings.Contains(d.Help, "spawn") {
						t.Fatalf("unexpected diagnostic: %+v", d)
					}
					delete(wantLines, d.Line)
					if d.Line > 17 && (d.PreviousFile != file || (d.PreviousLine != 4 && d.PreviousLine != 8)) {
						t.Fatalf("lost transitive operation witness: %+v", d)
					}
				}
			})
		}
	}
}
