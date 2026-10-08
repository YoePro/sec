package readiness_test

import (
	"errors"
	"os"
	"strconv"
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

// Rules: rules/types/types.md — Explicit conversions, int and uint; MD-012 §4.
func TestLegacyRuntimeIntegerConversionDomains(t *testing.T) {
	template, err := os.ReadFile("../../../testdata/codegen/runtime_integer_conversions/convert.sec.in")
	if err != nil {
		t.Fatal(err)
	}
	types := []string{"int", "uint", "int8", "uint8", "int32", "uint32", "int64", "uint64", "int128", "uint256"}
	for _, triple := range []string{"armv7-unknown-linux-gnueabihf", "x86_64-pc-linux-gnu"} {
		plan, err := targetplan.Plan(triple)
		if err != nil {
			t.Fatal(err)
		}
		for _, source := range types {
			for _, target := range types {
				t.Run(triple+"/"+source+"_to_"+target, func(t *testing.T) {
					// Independent bit-domain oracle: same-sign widening is safe; an unsigned
					// domain needs one extra signed bit; negative domains never fit unsigned.
					width := func(name string) int {
						if name == "int" || name == "uint" {
							return int(plan.PointerWidthBits)
						}
						bits, _ := strconv.Atoi(strings.TrimPrefix(strings.TrimPrefix(name, "uint"), "int"))
						return bits
					}
					sourceUnsigned, targetUnsigned := strings.HasPrefix(source, "uint"), strings.HasPrefix(target, "uint")
					safe := sourceUnsigned == targetUnsigned && width(target) >= width(source) || sourceUnsigned && !targetUnsigned && width(target) > width(source)
					text := strings.NewReplacer("@DECLARATIONS@", "", "@SOURCE@", source, "@TARGET@", target).Replace(string(template))
					parsed := parser.New(lexer.NewWithFile(text, "conversion.sec")).Parse()
					if parsed.HasErrors {
						t.Fatal(parsed.Diagnostics)
					}
					analyzer := sema.NewAnalyzerWithScalarPlan(plan)
					if errors := analyzer.Analyze(parsed.Program); len(errors) > 0 {
						t.Fatal(errors)
					}
					for backend, generate := range map[string]func(*ast.Program) (string, error){
						"llvm":     func(p *ast.Program) (string, error) { return llvm.GenerateAnalyzed(p, analyzer, triple) },
						"llvm_raw": func(p *ast.Program) (string, error) { return llvm.GenerateWithTriple(p, triple) },
						"mlir":     func(p *ast.Program) (string, error) { return mlir.GenerateAnalyzed(p, analyzer, triple) },
						"mlir_raw": func(p *ast.Program) (string, error) { return mlir.GenerateWithTriple(p, triple) },
					} {
						output, err := generate(parsed.Program)
						if safe {
							if err != nil || output == "" {
								t.Fatal(backend, "safe domain rejected", err)
							}
							if width(target) > width(source) {
								op := "sext"
								if sourceUnsigned {
									op = "zext"
								}
								if !strings.Contains(output, op) {
									t.Fatal(backend, "wrong extension", output)
								}
							}
						} else {
							var unsupported *semantic.UnsupportedFeatureError
							if output != "" || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "MD-012") || unsupported.Location.File != "conversion.sec" {
								t.Fatal(backend, "unsafe conversion emitted", output, err)
							}
						}
					}
				})
			}
		}
	}
}

// Rules: rules/types/types.md — Named types and Explicit conversions.
func TestLegacyRuntimeIntegerNominalAndASTRoutes(t *testing.T) {
	template, err := os.ReadFile("../../../testdata/codegen/runtime_integer_conversions/convert.sec.in")
	if err != nil {
		t.Fatal(err)
	}
	for _, triple := range []string{"armv7-unknown-linux-gnueabihf", "x86_64-pc-linux-gnu"} {
		plan, err := targetplan.Plan(triple)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range []struct {
			source, target string
			safe32, safe64 bool
		}{
			{"uint32", "int", false, true}, {"int64", "int", false, true},
			{"uint64", "uint", false, true}, {"int", "int32", true, false},
			{"int8", "uint", false, false},
		} {
			for _, astRoute := range []bool{false, true} {
				t.Run(triple+"/"+item.source+"_to_"+item.target+"/ast="+strconv.FormatBool(astRoute), func(t *testing.T) {
					text := strings.NewReplacer("@DECLARATIONS@", "type Source "+item.source+"\ntype Target "+item.target, "@SOURCE@", "Source", "@TARGET@", "Target").Replace(string(template))
					parsed := parser.New(lexer.NewWithFile(text, "nominal.sec")).Parse()
					if parsed.HasErrors {
						t.Fatal(parsed.Diagnostics)
					}
					analyzer := sema.NewAnalyzerWithScalarPlan(plan)
					if errors := analyzer.Analyze(parsed.Program); len(errors) > 0 {
						t.Fatal(errors)
					}
					if astRoute {
						for _, stmt := range parsed.Program.Statements {
							if fn, ok := stmt.(*ast.FunctionDeclaration); ok && fn.Name.Value == "Convert" {
								result := fn.Body.Statements[0].(*ast.ReturnStatement)
								call := result.Value.(*ast.CallExpression)
								result.Value = &ast.ConversionExpression{Token: call.Token, Type: &ast.TypeReference{Name: "Target", Token: call.Token}, Value: call.Arguments[0]}
							}
						}
					}
					safe := item.safe64
					if plan.PointerWidthBits == 32 {
						safe = item.safe32
					}
					for name, generate := range map[string]func(*ast.Program) (string, error){
						"llvm": func(p *ast.Program) (string, error) { return llvm.GenerateWithTriple(p, triple) },
						"mlir": func(p *ast.Program) (string, error) { return mlir.GenerateWithTriple(p, triple) },
					} {
						output, err := generate(parsed.Program)
						if safe {
							if err != nil || output == "" {
								t.Fatal(name, err)
							}
						} else {
							var unsupported *semantic.UnsupportedFeatureError
							if output != "" || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "MD-012") {
								t.Fatal(name, "unsafe nominal conversion", output, err)
							}
						}
					}
				})
			}
		}
	}
}

// Rules: types/types.md — Explicit conversions; MD-012 correction §4.
func TestLegacyNativeScalarConversionAudit(t *testing.T) {
	template, err := os.ReadFile("../../../testdata/codegen/runtime_integer_conversions/convert.sec.in")
	if err != nil {
		t.Fatal(err)
	}
	for _, triple := range []string{"armv7-unknown-linux-gnueabihf", "x86_64-pc-linux-gnu"} {
		plan, err := targetplan.Plan(triple)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range []struct {
			source, target string
			safe32, safe64 bool
		}{
			{"int", "decimal", true, true}, {"uint", "decimal", true, false},
			{"decimal", "int", false, false}, {"decimal", "uint", false, false},
			{"float64", "int", false, false}, {"float32", "uint", false, false},
		} {
			t.Run(triple+"/"+item.source+"_to_"+item.target, func(t *testing.T) {
				text := strings.NewReplacer("@DECLARATIONS@", "", "@SOURCE@", item.source, "@TARGET@", item.target).Replace(string(template))
				parsed := parser.New(lexer.NewWithFile(text, "scalar.sec")).Parse()
				if parsed.HasErrors {
					t.Fatal(parsed.Diagnostics)
				}
				analyzer := sema.NewAnalyzerWithScalarPlan(plan)
				semaErrors := analyzer.Analyze(parsed.Program)
				if strings.HasPrefix(item.source, "float") {
					if len(semaErrors) == 0 || !strings.Contains(semaErrors[0].Message, "cannot convert") {
						t.Fatal("missing conversion relation was not diagnosed", semaErrors)
					}
				} else if len(semaErrors) > 0 {
					t.Fatal(semaErrors)
				}
				safe := item.safe64
				if plan.PointerWidthBits == 32 {
					safe = item.safe32
				}
				for name, generate := range map[string]func(*ast.Program) (string, error){
					"llvm": func(p *ast.Program) (string, error) { return llvm.GenerateWithTriple(p, triple) },
					"mlir": func(p *ast.Program) (string, error) { return mlir.GenerateWithTriple(p, triple) },
				} {
					output, err := generate(parsed.Program)
					if safe {
						if err != nil || output == "" {
							t.Fatal(name, err)
						}
						if item.source == "uint" && plan.PointerWidthBits == 32 && !strings.Contains(output, "zext") {
							t.Fatal(name, "unsigned decimal coefficient sign extended", output)
						}
					} else {
						var unsupported *semantic.UnsupportedFeatureError
						if output != "" || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "MD-012") {
							t.Fatal(name, "unsafe scalar conversion", output, err)
						}
					}
				}
			})
		}
	}
}
