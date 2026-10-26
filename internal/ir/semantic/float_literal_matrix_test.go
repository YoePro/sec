package semantic

import (
	"math"
	"math/big"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Parser convenience float64 values must never become the literal payload.
// A rational oracle verifies the canonical bits and sign through Sema and IR
// on both selected platform widths, with exact lexeme/source preservation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§11–19,82,85;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md — §2.
func TestFloatLiteralSourceIRMatrix(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/float_literal_matrix.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), "float_literal_matrix.sec"))
		parsed := p.Parse()
		if parsed.HasErrors {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errs := a.Analyze(parsed.Program); len(errs) != 0 {
			t.Fatal(errs)
		}
		module, err := Build(parsed.Program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{"float_literal_matrix.sec"}})
		if err != nil {
			t.Fatal(err)
		}
		sources := map[string]*ast.FunctionDeclaration{}
		for _, statement := range parsed.Program.Statements {
			if fn, ok := statement.(*ast.FunctionDeclaration); ok {
				sources[fn.Name.Value] = fn
			}
		}
		checked := 0
		for _, fn := range module.Functions {
			source := sources[fn.Name]
			if source == nil || fn.Name == "Identity32" || fn.Name == "Contexts" {
				continue
			}
			expression := source.Body.Statements[0].(*ast.ReturnStatement).Value
			negative := false
			if prefix, ok := expression.(*ast.PrefixExpression); ok {
				negative = prefix.Operator == "-"
				expression = prefix.Right
			}
			switch expression.(type) {
			case *ast.FloatLiteral, *ast.IntegerLiteral:
			default:
				t.Fatalf("%s: fixture no longer a numeric literal", fn.Name)
			}
			literalToken := expressionToken(expression)
			raw := literalToken.Lexeme
			text, _ := ast.SplitNumericLiteralSuffix(raw)
			text = ast.NormalizeNumericLiteralLexeme(text)
			exact, ok := new(big.Rat).SetString(text)
			if !ok {
				t.Fatalf("oracle cannot parse %s", raw)
			}
			typ, _ := module.Types.Lookup(fn.ReturnType)
			bits := typ.BitWidth
			if bits == 0 {
				bits = width
			}
			var number float64
			if bits == 32 {
				v, _ := exact.Float32()
				number = float64(v)
			} else {
				number, _ = exact.Float64()
			}
			if negative {
				number = -number
			}
			want := math.Float64bits(number)
			if bits == 32 {
				want = uint64(math.Float32bits(float32(number)))
			}
			var got uint64
			constants, negations := 0, 0
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					switch op.Kind {
					case OpConstFloat:
						constants++
						if op.Float == nil || !op.Float.Canonical || op.Float.Format != bits || op.FloatLexeme != raw || op.Location.Line != literalToken.Line || op.Location.Column != literalToken.Column {
							t.Fatalf("%s: payload %+v expected format %d, lexeme %q/%q, location %+v/token %+v", fn.Name, op.Float, bits, op.FloatLexeme, raw, op.Location, literalToken)
						}
						got = op.Float.Bits
					case OpFloatNeg:
						negations++
						got ^= uint64(1) << (bits - 1)
					}
				}
			}
			if constants != 1 || (negations == 1) != negative || negations > 1 || got != want {
				t.Errorf("%s plan %d: got %#x (%d constants, %d negations), want %#x", fn.Name, width, got, constants, negations, want)
			}
			checked++
		}
		if checked != 63 {
			t.Fatalf("checked %d literal cases, want 63", checked)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
	}
}

// Source overflow never becomes infinity in a constant. The source pipeline
// may reject it during parsing, contextual Sema checking or IR construction.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§16,19,85.
func TestFloatLiteralSourceOverflow(t *testing.T) {
	for _, file := range []string{"float_literal_overflow32_invalid.sec", "float_literal_overflow64_invalid.sec"} {
		data, err := os.ReadFile("../../../testdata/semantic_ir/" + file)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			p := parser.New(lexer.NewWithFile(string(data), file))
			parsed := p.Parse()
			if parsed.HasErrors {
				continue
			}
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errs := a.Analyze(parsed.Program); len(errs) != 0 {
				continue
			}
			_, err := Build(parsed.Program, a, BuildOptions{RequestedModule: "main", SourceFiles: []string{file}})
			if err == nil || !strings.Contains(err.Error(), "finite range") {
				t.Fatalf("%s width %d: overflow accepted or misclassified: %v", file, width, err)
			}
		}
	}
}
