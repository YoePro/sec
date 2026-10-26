package sema

import (
	"math"
	"os"
	"strconv"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Source literal spelling survives parsing and context shaping, and CTE rounds
// directly at the target width, including ties, subnormals and signed zero.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§11–19,82,85;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md — §2.
func TestFloatLiteralSourceAndCTEMatrix(t *testing.T) {
	data, err := os.ReadFile("../../testdata/semantic_ir/float_literal_matrix.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), "float_literal_matrix.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		for _, stmt := range program.Statements {
			fn, ok := stmt.(*ast.FunctionDeclaration)
			if !ok || fn.Name.Value == "Identity32" {
				continue
			}
			if fn.Name.Value == "Contexts" {
				for expression, resolved := range a.expressionTypes {
					switch expression.(type) {
					case *ast.FloatLiteral, *ast.IntegerLiteral:
						if expressionToken(expression).Line > fn.Token.Line && resolved.Name != "float32" {
							t.Errorf("context literal %s typed %s, want float32", expression.String(), resolved.Name)
						}
					}
				}
				continue
			}
			result := fn.Body.Statements[0].(*ast.ReturnStatement).Value
			typ := a.types[fn.ReturnType.Name]
			resolved, ok := a.ResolvedTypeOf(result)
			if !ok || resolved.Name != typ.Name {
				t.Errorf("%s shaped to %s, want %s", fn.Name.Value, resolved.Name, typ.Name)
				continue
			}
			value, outcome := a.semanticCompileTimeConstantVisiting(result, map[string]bool{}, typ)
			if outcome != compileTimeEvaluated {
				t.Errorf("%s: CTE unavailable (%d)", fn.Name.Value, outcome)
				continue
			}
			exact, lexeme, ok := exactNumericConstant(result)
			if !ok {
				t.Fatal(fn.Name.Value)
			}
			var expected float64
			if typ.FloatBits == 32 {
				v, _ := exact.Float32()
				expected = float64(v)
			} else {
				expected, _ = exact.Float64()
			}
			if expected == 0 && len(lexeme) > 0 && lexeme[0] == '-' {
				expected = math.Copysign(0, -1)
			}
			got, err := strconv.ParseFloat(value.Lexeme, typ.FloatBits)
			if err != nil || math.Float64bits(got) != math.Float64bits(expected) {
				t.Errorf("%s (%d): CTE %q (%v), want %g bits %x", fn.Name.Value, width, value.Lexeme, err, expected, math.Float64bits(expected))
			}
		}
	}
}

// Explicit non-floating family suffixes cannot silently become float literals.
// Rules: rules/types/types.md — "Numeric literal suffixes", "Context shaping".
func TestFloatLiteralFamilyBoundaries(t *testing.T) {
	data, err := os.ReadFile("../../testdata/semantic_ir/float_literal_family_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 3 {
		t.Fatalf("expected three family mismatches, got %v", errors)
	}
}
