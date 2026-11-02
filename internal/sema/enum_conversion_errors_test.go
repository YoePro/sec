package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// Enum narrowing uses the public conversion channel, including Result carrier
// typing, ordinary try propagation, local handling and explicit error mapping.
// Rules: rules/declarations/enums.md — §12.1;
// rules/types/types.md — "Explicit conversions", "Exact integer conversion proof";
// rules/errors/errorhandling.md — §§9–11.
func TestEnumConversionCanonicalErrorChannel(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_conversion_errors/valid.sec")
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
	fallible, exact, tries := 0, 0, 0
	_ = walk.Inspect(program, func(node any) error {
		if expr, ok := node.(*ast.TryExpression); ok {
			tries++
			fact, found := a.ResolvedTryOf(expr)
			if !found || !sameConcreteType(fact.ErrorType, a.Types()["ConversionError"]) {
				t.Fatalf("try lost conversion channel: %+v", fact)
			}
		}
		call, ok := node.(*ast.CallExpression)
		if !ok {
			return nil
		}
		conversion, ok := a.ResolvedEnumConversionOf(call)
		if !ok || conversion.Kind != ResolvedEnumToInteger {
			return nil
		}
		result, ok := a.ResolvedTypeOf(call)
		if !ok {
			t.Fatal("missing conversion result")
		}
		if conversion.Fallible {
			fallible++
			if result.Kind != ResultType || len(result.TypeArgs) != 2 || result.TypeArgs[0].Name != "uint8" || !sameConcreteType(result.TypeArgs[1], a.Types()["ConversionError"]) || conversion.Proof.Proven() {
				t.Fatalf("wrong checked-conversion channel: %+v / %+v", result, conversion)
			}
		} else {
			exact++
			if result.Kind == ResultType || !conversion.Proof.Proven() {
				t.Fatalf("exact conversion became fallible: %+v / %+v", result, conversion)
			}
		}
		return nil
	})
	if fallible != 5 || exact != 2 || tries != 4 {
		t.Fatalf("conversions: fallible=%d exact=%d", fallible, exact)
	}
}

// Unmapped propagation into ArithmeticError is rejected. Known-invalid values
// remain compile-time errors rather than being demoted into a runtime Result.
// Rules: rules/types/types.md — "Explicit conversions" (MD-012);
// rules/errors/errorhandling.md — ordinary Result propagation compatibility.
func TestEnumConversionRejectsLegacyErrorChannel(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_conversion_errors/invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 4 {
		t.Fatalf("errors = %v", errors)
	}
	for i, want := range []string{"Result[uint8, ConversionError]", "ConversionError", "Result[uint8, ConversionError]", "300 overflows uint8"} {
		if !strings.Contains(errors[i].Message, want) {
			t.Errorf("error %d = %v; want %s", i, errors[i], want)
		}
	}
}

// Core-less analysis and loader-proven source declarations agree on the public
// union inventory. Loading core refines Contract to the actual ContractError
// payload instead of retaining the standalone placeholder.
// Rules: sec/core/error.sec; MD-012 correction — §§3.1–3.3;
// rules/types/contracts.md — "Public conversion and contract errors".
func TestEnumConversionErrorCoreDeclaration(t *testing.T) {
	data, err := os.ReadFile("../../sec/core/error.sec")
	if err != nil {
		t.Fatal(err)
	}
	const coreFile = "sec/core/error.sec"
	p := parser.New(lexer.NewWithFile(string(data), coreFile))
	core := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	statements := []ast.Statement{core.Statements[0]}
	for _, stmt := range core.Statements {
		switch decl := stmt.(type) {
		case *ast.EnumDeclaration:
			if decl.Name.Value == "ContractKind" {
				statements = append(statements, stmt)
			}
		case *ast.TypeDeclStatement:
			if decl.Name.Value == "ContractError" || decl.Name.Value == "ConversionError" {
				statements = append(statements, stmt)
			}
		}
	}
	if len(statements) != 4 {
		t.Fatalf("missing canonical core declarations: %d", len(statements))
	}
	data, err = os.ReadFile("../../testdata/sema/enum_conversion_errors/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p = parser.New(lexer.NewWithFile(string(data), "enum-conversion.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	program.Statements = append(statements, program.Statements...)
	program.SourceProvenance = map[string]ast.SourceProvenance{coreFile: ast.SourceCore}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	standalone := NewAnalyzer().Types()["ConversionError"]
	loaded := a.Types()["ConversionError"]
	if !sameConcreteType(standalone, loaded) || loaded.Kind != UnionType || !loaded.ErrorAssignable || len(loaded.UnionVariants) != 6 || len(standalone.UnionVariants) != 6 {
		t.Fatalf("core identity/shape diverged: %+v / %+v", standalone, loaded)
	}
	for i, variant := range loaded.UnionVariants {
		if variant.Name != standalone.UnionVariants[i].Name {
			t.Fatalf("variant %d differs: %s", i, variant.Name)
		}
	}
	contract := loaded.UnionVariants[5]
	if contract.Name != "Contract" || contract.Payload == nil || contract.Payload.Name != "ContractError" || contract.Payload.Kind != UnionType {
		t.Fatalf("source payload lost: %+v", contract)
	}
}
