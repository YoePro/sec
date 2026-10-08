package readiness_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/codegen/llvm"
	"sec/internal/codegen/mlir"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// wideFixture keeps accepted Sec programs in testdata with exact source tokens.
// Rules: rules/types/types.md — active wide integer and decimal families.
func wideFixture(t *testing.T, fixture, typ, operation string) *ast.Program {
	t.Helper()
	path := "../../../testdata/codegen/wide_legacy/" + fixture
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := strings.ReplaceAll(strings.ReplaceAll(string(data), "TYPE", typ), "OP", operation)
	parsed := parser.New(lexer.NewWithFile(source, path)).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	return parsed.Program
}

// TestLegacyWideCheckedOperations prevents unchecked wide overflow on every
// raw and analyzed entry point, including compound assignments and negation.
// Rules: rules/foundations/operators.md — Checked integer arithmetic;
// rules/types/types.md — Integer types and Exact decimal types.
func TestLegacyWideCheckedOperations(t *testing.T) {
	for _, typ := range []string{"int128", "uint128", "int256", "uint256", "decimal128"} {
		for _, fixture := range []string{"operation.sec.in", "compound.sec.in", "negate.sec.in"} {
			for _, op := range []string{"+", "-", "*", "/", "%"} {
				if fixture == "negate.sec.in" && (op != "-" || strings.HasPrefix(typ, "uint")) {
					continue
				}
				if typ == "decimal128" && op == "%" {
					continue
				}
				t.Run(typ+"/"+fixture+"/"+op, func(t *testing.T) {
					program := wideFixture(t, fixture, typ, op)
					plan, err := targetplan.Plan("x86_64-pc-linux-gnu")
					if err != nil {
						t.Fatal(err)
					}
					analyzer := sema.NewAnalyzerWithScalarPlan(plan)
					if problems := analyzer.Analyze(program); len(problems) > 0 {
						t.Fatal(problems)
					}
					for name, generate := range map[string]func(*ast.Program) (string, error){
						"llvm": llvm.Generate, "llvm-generator": llvm.NewGenerator().Generate,
						"llvm-triple":   func(p *ast.Program) (string, error) { return llvm.GenerateWithTriple(p, plan.LLVMTriple) },
						"llvm-analyzed": func(p *ast.Program) (string, error) { return llvm.GenerateAnalyzed(p, analyzer, plan.LLVMTriple) },
						"mlir":          (&mlir.Generator{}).Generate,
						"mlir-triple":   func(p *ast.Program) (string, error) { return mlir.GenerateWithTriple(p, plan.LLVMTriple) },
						"mlir-analyzed": func(p *ast.Program) (string, error) { return mlir.GenerateAnalyzed(p, analyzer, plan.LLVMTriple) },
					} {
						output, err := generate(program)
						var failure *semantic.UnsupportedFeatureError
						if output != "" || !errors.As(err, &failure) || failure.Location.Line == 0 || failure.Location.File == "" {
							t.Fatalf("%s: %q %v", name, output, err)
						}
					}
				})
			}
		}
	}
}

// TestLegacyWideSupportedCarriers ensures safety gates retain lossless storage,
// calls, signed/unsigned comparisons, and exact literals on 32/64-bit targets.
// Rules: rules/types/types.md — explicit widths, Context shaping, Exact decimal types.
func TestLegacyWideSupportedCarriers(t *testing.T) {
	for _, triple := range []string{"armv7-unknown-linux-gnueabihf", "x86_64-pc-linux-gnu"} {
		for _, typ := range []string{"int128", "uint128", "int256", "uint256", "decimal128"} {
			for _, fixture := range []string{"identity.sec.in", "comparison.sec.in", "left_literal.sec.in"} {
				program := wideFixture(t, fixture, typ, "")
				for name, generate := range map[string]func(*ast.Program) (string, error){
					"llvm": func(p *ast.Program) (string, error) { return llvm.GenerateWithTriple(p, triple) },
					"mlir": func(p *ast.Program) (string, error) { return mlir.GenerateWithTriple(p, triple) },
				} {
					if typ == "decimal128" && (name == "llvm" || (fixture == "comparison.sec.in" || fixture == "left_literal.sec.in")) {
						continue
					}
					output, err := generate(program)
					if err != nil || output == "" {
						t.Fatalf("%s/%s/%s: %q %v", name, typ, fixture, output, err)
					}
					if fixture == "comparison.sec.in" || fixture == "left_literal.sec.in" {
						predicate := "slt"
						if strings.HasPrefix(typ, "uint") {
							predicate = "ult"
						}
						if !strings.Contains(output, predicate) {
							t.Fatal(output)
						}
					}
				}
			}
		}
		program := wideFixture(t, "exact_constants.sec", "", "")
		for _, generate := range []func(*ast.Program) (string, error){func(p *ast.Program) (string, error) { return llvm.GenerateWithTriple(p, triple) }, func(p *ast.Program) (string, error) { return mlir.GenerateWithTriple(p, triple) }} {
			output, err := generate(program)
			if err != nil {
				t.Fatal(err)
			}
			for _, literal := range []string{"-170141183460469231731687303715884105728", "115792089237316195423570985008687907853269984665640564039457584007913129639935"} {
				if !strings.Contains(output, literal) {
					t.Fatal(output)
				}
			}
		}
	}
}

// TestLegacyLLVMImportedWideDecimal cannot erase a decimal128 alias when its
// defining declarations are absent from the output AST.
// Rules: rules/types/types.md — Named types, Exact decimal types.
func TestLegacyLLVMImportedWideDecimal(t *testing.T) {
	program := wideFixture(t, "decimal_alias.sec", "", "")
	analyzer := sema.NewAnalyzer()
	if problems := analyzer.Analyze(program); len(problems) > 0 {
		t.Fatal(problems)
	}
	root := *program
	root.Statements = nil
	for _, stmt := range program.Statements {
		if _, decl := stmt.(*ast.TypeDeclStatement); !decl {
			root.Statements = append(root.Statements, stmt)
		}
	}
	output, err := llvm.GenerateAnalyzed(&root, analyzer, "x86_64-pc-linux-gnu")
	var failure *semantic.UnsupportedFeatureError
	if output != "" || !errors.As(err, &failure) || !strings.Contains(failure.Feature, "decimal128") || failure.Location.Line == 0 {
		t.Fatal(output, err)
	}
}

// TestLegacyWideShiftAndConstantGates checks count/overflow failure paths and
// the raw MLIR constant-folding route, which can bypass expression emission.
// Rules: rules/foundations/operators.md — Shifts, Checked integer arithmetic.
func TestLegacyWideShiftAndConstantGates(t *testing.T) {
	for _, typ := range []string{"int128", "uint128", "int256", "uint256"} {
		for _, op := range []string{"<<", ">>"} {
			p := wideFixture(t, "shift.sec.in", typ, op)
			a := sema.NewAnalyzer()
			if problems := a.Analyze(p); len(problems) > 0 {
				t.Fatal(problems)
			}
			out, err := mlir.GenerateAnalyzed(p, a, "x86_64-pc-linux-gnu")
			var failure *semantic.UnsupportedFeatureError
			if out != "" || !errors.As(err, &failure) || failure.Location.Line == 0 {
				t.Fatal(out, err)
			}
		}
	}
	p := wideFixture(t, "overflow_constant.sec", "", "")
	out, err := mlir.GenerateWithTriple(p, "x86_64-pc-linux-gnu")
	var failure *semantic.UnsupportedFeatureError
	if out != "" || !errors.As(err, &failure) || failure.Location.Line == 0 {
		t.Fatal(out, err)
	}
}

// TestLegacyImportedWideInteger keeps nominal representation facts mandatory
// when imported declarations have not been supplied to the legacy output AST.
// Rules: rules/types/types.md — Named types, Integer types.
func TestLegacyImportedWideInteger(t *testing.T) {
	for _, typ := range []string{"int128", "uint128", "int256", "uint256"} {
		p := wideFixture(t, "alias.sec.in", typ, "")
		a := sema.NewAnalyzer()
		if problems := a.Analyze(p); len(problems) > 0 {
			t.Fatal(problems)
		}
		root := *p
		root.Statements = nil
		for _, stmt := range p.Statements {
			if _, decl := stmt.(*ast.TypeDeclStatement); !decl {
				root.Statements = append(root.Statements, stmt)
			}
		}
		for _, generate := range []func(*ast.Program) (string, error){func(p *ast.Program) (string, error) { return llvm.GenerateAnalyzed(p, a, "x86_64-pc-linux-gnu") }, func(p *ast.Program) (string, error) { return mlir.GenerateAnalyzed(p, a, "x86_64-pc-linux-gnu") }} {
			out, err := generate(&root)
			var failure *semantic.UnsupportedFeatureError
			if out != "" || !errors.As(err, &failure) || failure.Location.Line == 0 {
				t.Fatal(out, err)
			}
		}
	}
}

// TestLegacyWideAggregateCompound checks both represented aggregate write
// routes, ensuring neither bypasses the shared operation prerequisite.
// Rules: rules/foundations/operators.md — Checked integer arithmetic, compound forms.
func TestLegacyWideAggregateCompound(t *testing.T) {
	for _, fixture := range []string{"element_compound.sec.in", "field_compound.sec.in"} {
		p := wideFixture(t, fixture, "int128", "")
		a := sema.NewAnalyzer()
		if problems := a.Analyze(p); len(problems) > 0 {
			t.Fatal(problems)
		}
		for _, generate := range []func(*ast.Program) (string, error){func(p *ast.Program) (string, error) { return mlir.GenerateWithTriple(p, "x86_64-pc-linux-gnu") }, func(p *ast.Program) (string, error) { return mlir.GenerateAnalyzed(p, a, "x86_64-pc-linux-gnu") }} {
			out, err := generate(p)
			var failure *semantic.UnsupportedFeatureError
			if out != "" || !errors.As(err, &failure) || failure.Location.Line == 0 {
				t.Fatal(out, err)
			}
		}
	}
}
