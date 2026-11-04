package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Accepted Result projections publish receiver identity independently from
// source spelling; query copies and reanalysis cannot alter the stored Place.
// Rules: rules/errors/errorhandling.md §§6.1,29;
// rules/compiler/compiler_analysis.md §55.
func TestReceiverMoveFactsDetachedAndReset(t *testing.T) {
	parse := func(name string) *ast.Program {
		t.Helper()
		source, err := os.ReadFile("../../testdata/lsp/move_hints/" + name)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(string(source)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		return program
	}
	program := parse("valid.sec")
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	var calls []*ast.CallExpression
	ast.Visit(program, func(node any) bool {
		if call, ok := node.(*ast.CallExpression); ok {
			if source, consumes := a.ResolvedReceiverMoveOf(call); consumes {
				if source.RootID == 0 || source.String() != "value" && source.String() != "value.Item" {
					t.Fatalf("missing canonical source: %+v", source)
				}
				before, _ := a.ResolvedReceiverMoveOf(call)
				source.Root = "mutated"
				if len(source.Projections) != 0 {
					source.Projections[0].Name = "mutated"
				}
				if after, _ := a.ResolvedReceiverMoveOf(call); !reflect.DeepEqual(before, after) {
					t.Fatal("query mutated stored receiver source")
				}
				calls = append(calls, call)
			}
		}
		return true
	})
	if len(calls) != 3 {
		t.Fatalf("got %d receiver moves, want three accepted named/projected receivers", len(calls))
	}
	if errors := a.Analyze(program); len(errors) != 0 || len(a.receiverMoves) != 3 {
		t.Fatal("reanalyzing a snapshot changed receiver facts", errors)
	}
	if errors := a.Analyze(parse("invalid.sec")); len(errors) == 0 || len(a.receiverMoves) != 1 {
		t.Fatal("rejected calls published moves", errors, a.receiverMoves)
	}
	for _, call := range calls {
		if _, old := a.ResolvedReceiverMoveOf(call); old {
			t.Fatal("old snapshot retained after reanalysis")
		}
	}
}
