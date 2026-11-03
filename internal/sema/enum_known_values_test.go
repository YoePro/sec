package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// Statically established enum values use exact numeric checking; runtime
// bindings and ordinary calls never inherit unrelated constant evidence.
// Rules: rules/declarations/enums.md — §§12.1–12.3;
// rules/types/types.md — Exact integer conversion proof (MD-104).
func TestKnownEnumValuesRetainExactProofs(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_known_values/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		exact, fallible := 0, 0
		_ = walk.Inspect(program, func(node any) error {
			call, ok := node.(*ast.CallExpression)
			if !ok {
				return nil
			}
			fact, ok := a.ResolvedEnumConversionOf(call)
			if !ok || fact.Kind != ResolvedEnumToInteger {
				return nil
			}
			if fact.Fallible {
				fallible++
				return nil
			}
			exact++
			if fact.Proof.Basis != IntegerConversionConstant || fact.Proof.SourceMin == nil || fact.Proof.SourceMin.Cmp(fact.Proof.SourceMax) != 0 {
				t.Fatalf("%d-bit constant proof lost after scope: %+v", width, fact)
			}
			return nil
		})
		if exact != 8 || fallible != 4 {
			t.Fatalf("%d-bit conversions exact=%d fallible=%d", width, exact, fallible)
		}
	}
}

// Known invalid values fail statically, including through an immutable alias
// or validated enum constructor; runtime Result does not disguise the error.
// Rules: rules/declarations/enums.md — §12.1; types.md — Explicit conversions (MD-012).
func TestKnownEnumValuesRejectOverflow(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_known_values/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 4 {
		t.Fatalf("errors: %v", errors)
	}
	for i, want := range []string{"300 overflows uint8", "300 overflows uint8", "-129 overflows int8", "256 overflows uint8"} {
		if !strings.Contains(errors[i].Message, want) {
			t.Errorf("%d: %v; want %s", i, errors[i], want)
		}
	}
}
