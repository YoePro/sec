package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestCompareExchangeResultSurface checks the nominal generic core enum,
// construction, payload binding, exhaustiveness, defaults and non-error status.
// Rules: rules/concurrency/atomics.md — §§4,6,12,17,43–45;
// rules/types/default_values.md — Enums.
func TestCompareExchangeResultSurface(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, test := range []struct {
			file   string
			errors int
		}{
			{"compare_exchange_result_valid.sec", 0},
			{"compare_exchange_result_invalid.sec", 13},
			{"compare_exchange_result_spoof_invalid.sec", 1},
			{"lowering_valid.sec", 0},
		} {
			data, err := os.ReadFile("../../testdata/sema/atomics/compare_exchange_result/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), test.file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: "test", PointerWidthBits: width, Endianness: layout.LittleEndian})
			errors := a.Analyze(program)
			if len(errors) != test.errors {
				t.Fatalf("%d %s: %d: %v", width, test.file, len(errors), errors)
			}
			for _, diagnostic := range errors {
				if strings.Contains(diagnostic.Message, "union variant CompareExchangeResult") {
					t.Fatal("source enum reported as a union", diagnostic)
				}
			}
			typ := a.Types()["CompareExchangeResult"]
			if !isCompareExchangeResult(typ) || typ.ErrorAssignable || len(typ.GenericParameters) != 1 || len(typ.UnionVariants) != 2 || typ.UnionVariants[0].Payload != nil || typ.UnionVariants[1].Payload == nil || typ.UnionVariants[1].Payload.Name != "T" {
				t.Fatal(typ)
			}
			if test.errors != 0 {
				continue
			}
			for _, statement := range program.Statements {
				fn, ok := statement.(*ast.FunctionDeclaration)
				if !ok {
					continue
				}
				if fn.Name.Value == "Observe" {
					binding, ok := fn.Body.Statements[0].(*ast.LetStatement)
					if !ok {
						continue
					}
					call := binding.Value.(*ast.CallExpression)
					result, ok := a.ResolvedTypeOf(call)
					if !ok || !isCompareExchangeResult(result) || result.UnionVariants[1].Payload.Name != "Count" {
						t.Fatal(result, ok)
					}
				}
			}
		}
	}
}

// TestCompareExchangeResultRejectsMalformedCore refuses a different payload or
// declaration kind even with loader-proven provenance; the reserved fallback
// remains canonical for recovery and later diagnostics.
// Rules: rules/concurrency/atomics.md — §§4(7),6(1–5).
func TestCompareExchangeResultRejectsMalformedCore(t *testing.T) {
	for _, name := range []string{"core_payload_invalid.sec", "core_shape_invalid.sec"} {
		file := "../../testdata/sema/atomics/compare_exchange_result/" + name
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
		a := NewAnalyzer()
		if errors := a.Analyze(program); len(errors) != 1 {
			t.Fatal(name, errors)
		}
		if !isCompareExchangeResult(a.Types()["CompareExchangeResult"]) {
			t.Fatal(a.Types()["CompareExchangeResult"])
		}
	}
}

// TestCompareExchangeResultCoreDeclaration gives standalone and loader-proven
// source registration the same identity while retaining each variant's token.
// Rules: rules/concurrency/atomics.md — §§4(4–7),6,44(4–5).
func TestCompareExchangeResultCoreDeclaration(t *testing.T) {
	const file = "../../sec/core/atomic.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	typ := a.Types()["CompareExchangeResult"]
	if !sameConcreteType(typ, builtinCompareExchangeResultType()) {
		t.Fatal(typ)
	}
	for _, variant := range typ.UnionVariants {
		if variant.Token.File != file {
			t.Fatal(variant)
		}
	}
}
