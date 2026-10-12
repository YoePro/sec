package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestBorrowAuthorityDiagnostics distinguishes ownership availability from
// borrow authority for read/write/reborrow/move/discard and keeps both locations.
// Rules: rules/memory/borrowing.md — §§6(1–7),32(6–8),33(1).
func TestBorrowAuthorityDiagnostics(t *testing.T) {
	for _, test := range []struct {
		name  string
		count int
	}{{"borrow_authority_invalid.sec", 7}, {"borrow_authority_valid.sec", 0}} {
		data, err := os.ReadFile("../../testdata/tooling/" + test.name)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), test.name))
		result := p.Parse()
		if result.HasErrors {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		errors := a.Analyze(result.Program)
		if len(errors) != test.count {
			t.Fatalf("%s: %+v", test.name, errors)
		}
		for _, statement := range result.Program.Statements {
			function, ok := statement.(*ast.FunctionDeclaration)
			if !ok {
				continue
			}
			for _, statement := range function.Body.Statements {
				branch, ok := statement.(*ast.IfStatement)
				if !ok {
					continue
				}
				query := branch.Condition.(*ast.AvailabilityExpression)
				fact, ok := a.ResolvedAvailabilityTestOf(query)
				if !ok || !fact.StaticallyKnown || !fact.Value {
					t.Fatalf("borrow changed ownership availability: %+v", fact)
				}
			}
		}
		for _, diagnostic := range errors {
			if !strings.Contains(diagnostic.Help, "is available (still owned), but an active borrow blocks this access") || !strings.Contains(diagnostic.Help, "not borrow access authority") || diagnostic.PreviousLine != diagnostic.Line-1 || diagnostic.PreviousFile != test.name {
				t.Fatalf("lost availability/authority distinction or origin: %+v", diagnostic)
			}
		}
	}
}

// TestBorrowAuthorityLoopCarriedHelp preserves uncertainty about a borrow from
// an earlier iteration while still explaining proven ownership availability.
// Rules: rules/memory/borrowing.md — §32(4,7).
func TestBorrowAuthorityLoopCarriedHelp(t *testing.T) {
	a := NewAnalyzer()
	place := Place{Root: "value"}
	origin := lexer.Token{File: "loop.sec", Line: 2, Column: 1}
	a.borrows = map[string][]borrowRecord{"value": {{Kind: mutableBorrow, Holder: "holder", Place: place, Token: origin, LoopCarried: true}}}
	if !a.checkBorrowedReadPlace(place, lexer.Token{File: "loop.sec", Line: 3, Column: 1}) {
		t.Fatal("loop-carried borrow was ignored")
	}
	errors := a.errors
	if len(errors) != 1 || !strings.Contains(errors[0].Help, "value is available (still owned)") || !strings.Contains(errors[0].Help, "may still block this access") || strings.Contains(errors[0].Help, "an active borrow blocks") {
		t.Fatalf("lost independent ownership/borrow proof: %+v", errors)
	}
}
