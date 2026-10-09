package contractdiagnostics_test

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: rules/types/contracts.md — Ordered membership, Composition, Diagnostics;
// rules/tooling/diagnostics.md — §§8(2),9(7).
// A duplicate keeps its original member and the defining contract; inherited
// membership failure links a base member to both the new range and original list.
func TestContractRelatedLocations(t *testing.T) {
	program := &ast.Program{}
	for _, name := range []string{"definitions.sec", "use_invalid.sec"} {
		data, err := os.ReadFile("../../../testdata/contracts/locations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		result := parser.New(lexer.NewWithFile(string(data), name)).Parse()
		if result.HasErrors {
			t.Fatal(result.Diagnostics)
		}
		program.Statements = append(program.Statements, result.Program.Statements...)
	}
	errors := sema.NewAnalyzer().Analyze(program)
	if len(errors) != 2 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		related := e.RelatedLocations()
		if len(related) != 2 {
			t.Fatal(e, related)
		}
		switch e.ID {
		case diagnostics.DuplicateContractMembershipValue:
			if e.File != "definitions.sec" || e.Line != 3 || e.Column != 33 || related[0].File != "definitions.sec" || related[0].Column != 30 || related[1].File != "definitions.sec" || related[1].Column != 26 {
				t.Fatal(e, related)
			}
		case diagnostics.InvalidMembershipValue:
			if e.File != "definitions.sec" || e.Line != 4 || related[0].File != "use_invalid.sec" || related[0].Column != 29 || related[1].File != "definitions.sec" || related[1].Column != 18 {
				t.Fatal(e, related)
			}
		default:
			t.Fatal(e)
		}
		if related[1].EndColumn <= related[1].Column || related[1].Label != "contract declaration" {
			t.Fatal(related)
		}
	}
}

// Rules: rules/tooling/diagnostics.md — §§8(2),9(2,7).
// Compatibility links remain first and output normalization never mutates input.
func TestRelatedLocationNormalization(t *testing.T) {
	e := sema.Error{PreviousFile: "base.sec", PreviousLine: 2, PreviousColumn: 3, Related: []sema.RelatedLocation{
		{File: "base.sec", Line: 2, Column: 3, Label: "duplicate"},
		{File: "bad.sec", Line: 0, Column: 1},
		{File: "contract.sec", Line: 4, Column: 5, Label: "contract declaration"},
	}}
	links := e.RelatedLocations()
	if len(links) != 2 || links[0].Label != "previous declaration" {
		t.Fatal(links)
	}
	links[1].Label = "changed"
	if e.Related[2].Label != "contract declaration" {
		t.Fatal("normalization aliases source slice")
	}
}
