package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// A fallible enum conversion obeys ordinary Result must-use and try semantics
// in every value context; storing/returning/matching/explicit discard are legal
// Result uses and do not require an extra eager try at the conversion site.
// Rules: rules/declarations/enums.md — §12.1;
// rules/errors/errorhandling.md — §§7,9,12,13,14,26.
func TestEnumConversionOrdinaryTryHandling(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_conversion_try/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	tries := 0
	_ = walk.Inspect(program, func(node any) error {
		expr, ok := node.(*ast.TryExpression)
		if !ok {
			return nil
		}
		tries++
		fact, found := a.ResolvedTryOf(expr)
		if !found || fact.SuccessType.Name != "uint8" || fact.ErrorType.Name != "ConversionError" {
			t.Fatalf("lost ordinary try decision: %+v", fact)
		}
		return nil
	})
	if tries != 6 {
		t.Fatalf("try facts: %d", tries)
	}
}

// A try protects its own expression, not a following statement or another
// argument. Every unwrapped-value use and silently ignored carrier is rejected.
// Rules: rules/errors/errorhandling.md — §§7,13,14;
// rules/declarations/enums.md — §12.1.
func TestEnumConversionTryRejectsUnhandledContexts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_conversion_try/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 7 {
		t.Fatalf("errors: %v", errors)
	}
	lines := map[int]bool{}
	for _, err := range errors {
		lines[err.Line] = true
		if (err.Line == 8 || err.Line == 11) && (err.ID != diagnostics.UnhandledMustUseResult || err.Help != "use try, match, binding, return, or explicit discard") {
			t.Errorf("lost mandatory must-use diagnostic/repair: %+v", err)
		}
		if !strings.Contains(err.Message, "Result") {
			t.Errorf("lost carrier diagnostic: %v", err)
		}
	}
	for _, line := range []int{5, 6, 7, 8, 11, 15, 18} {
		if !lines[line] {
			t.Errorf("unhandled conversion accepted at line %d: %v", line, errors)
		}
	}
}
