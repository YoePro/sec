package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMutexReacquisition checks canonical identity, guard moves and lexical
// lifetime at every analysis budget, including Analyzer reuse.
// Rules: rules/concurrency/mutex.md — §§28,37,42–43.
func TestMutexReacquisition(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		a := NewAnalyzerWithDepth(depth)
		for _, test := range []struct {
			file   string
			errors int
		}{
			{"mutex_reacquisition_invalid.sec", 12},
			{"mutex_reacquisition_valid.sec", 0},
			{"mutex_reacquisition_invalid.sec", 12},
		} {
			data, err := os.ReadFile("../../testdata/sema/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), test.file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			errors := a.Analyze(program)
			if len(errors) != test.errors {
				t.Fatalf("%s depth %v: %v", test.file, depth, errors)
			}
			for _, diagnostic := range errors {
				if !strings.Contains(diagnostic.Message, "already locked by the current execution entity") || diagnostic.PreviousLine == 0 || diagnostic.PreviousLine >= diagnostic.Line || diagnostic.RelatedLocationLabel() != "previous acquisition; guard is still live" {
					t.Fatal(diagnostic)
				}
			}
		}
	}
}
