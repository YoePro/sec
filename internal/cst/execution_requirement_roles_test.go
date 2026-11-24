package cst

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Only complete parser-owned requirements acquire list roles; recovered clauses
// stay outside layout rewriting. Rules: rules/tooling/formatter.md §§5,16(13a),25.
func TestExecutionRequirementRoles(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/execution_requirements.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	document := Build(string(data), "")
	document.ApplyProgramRoles(program)
	count := 0
	for _, group := range document.Groups {
		if !document.Elements[group.Open].HasRole(ExecutionRequirementListOpen) {
			continue
		}
		count++
		if group.Close < 0 || !document.Elements[group.Open].HasRole(CommaListOpen) || !document.Elements[group.Close].HasRole(CommaListClose) {
			t.Fatal("require list lacks matching comma roles")
		}
	}
	if count != 9 {
		t.Fatalf("got %d lists, want 9", count)
	}
}
