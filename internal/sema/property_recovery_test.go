package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules:
//   - rules/compiler/parser_recovery.md — "Missing setter parameter"
//   - rules/compiler/parser_recovery.md — "Interaction with Sema"
func TestInvalidPropertySetterSkipsDependentSemanticDiagnostics(t *testing.T) {
	p := parser.New(lexer.New(`
module main

type Record struct {}

impl Record {
    property Value: int {
        set {
            missing = alsoMissing
        }

        get { return 0 }
    }
}
`))
	program := p.ParseProgram()
	if len(p.Errors()) != 1 {
		t.Fatalf("parser errors = %v, want the missing-parameter error only", p.Errors())
	}
	analyzer := NewAnalyzer()
	assertSemaErrors(t, analyzer.Analyze(program), nil)
}
