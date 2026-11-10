package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTaskYieldContextDiagnostics checks direct and transitive requirements at
// program, thread and test roots, preserving valid task calls and source witnesses.
// Rules: rules/concurrency/scheduling.md — §§7(2),22,50,57;
// rules/concurrency/cancellation.md — §§19–23; rules/tooling/diagnostics.md — §§8–12.
func TestTaskYieldContextDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"task_yield_context_valid.sec", 0},
		{"task_yield_context_invalid.sec", 7},
		{"task_yield_context_invalid_test.sec", 1},
	} {
		file := "../../testdata/sema/" + tc.file
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(source), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if len(errors) != tc.errors {
			t.Fatalf("%s: %v", tc.file, errors)
		}
		lines := strings.Split(string(source), "\n")
		for _, d := range errors {
			if d.ID != diagnostics.LogicalTaskContextRequired || d.Severity != diagnostics.SeverityError || !strings.Contains(d.Help, "spawn") || d.File != file || d.EndColumn <= d.Column {
				t.Fatalf("invalid context diagnostic: %+v", d)
			}
			if strings.Contains(d.Message, "because") {
				if d.PreviousFile != file || d.PreviousLine != 4 || d.RelatedLocationLabel() != "current-execution requirement introduced here" {
					t.Fatalf("lost requirement origin: %+v", d)
				}
			} else if !strings.Contains(lines[d.Line-1], "Task.Yield()") {
				t.Fatalf("direct diagnostic misplaced: %+v", d)
			}
		}
	}
}
