package sema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestEnumFixedWidthConversionDomains tests signed and unsigned boundary
// conversions without relying on target-sized int/uint. Constants, aliases
// and closed enum domains retain distinct exactness evidence.
// Rules: rules/declarations/enums.md — §12.1;
// rules/types/types.md — "Exact integer conversion proof" (MD-104).
func TestEnumFixedWidthConversionDomains(t *testing.T) {
	testEnumConversionDomains(t, map[string]enumDomainExpectation{
		"ClosedByte":      {"Closed", "uint8", false, IntegerConversionClosedEnumDomain, "0", "255"},
		"ClosedSigned":    {"Closed", "int8", true, "", "", ""},
		"WideByte":        {"Wide", "uint8", true, "", "", ""},
		"SignedByte":      {"Signed", "int8", false, IntegerConversionClosedEnumDomain, "-128", "127"},
		"SignedUnsigned":  {"Signed", "uint8", true, "", "", ""},
		"TooLowByte":      {"TooLow", "int8", true, "", "", ""},
		"WideFits":        {"Wide", "uint16", false, IntegerConversionClosedEnumDomain, "0", "256"},
		"KnownFits":       {"Wide", "uint8", false, IntegerConversionConstant, "0", "0"},
		"KnownHighFits":   {"Closed", "uint8", false, IntegerConversionConstant, "255", "255"},
		"KnownSignedFits": {"TooLow", "int8", false, IntegerConversionConstant, "127", "127"},
	})
}

// TestOpenEnumConversionUsesEntireBitDomain proves that undeclared patterns
// such as 255/65535 participate in runtime conversion even with only 0/1 named.
// Rules: rules/declarations/enums.md — §§10,12.1;
// rules/types/types.md — "Exact integer conversion proof";
// correction md080-md082-md096-md098-md104-md105-correction-20261018.md — §6.2(3).
func TestOpenEnumConversionUsesEntireBitDomain(t *testing.T) {
	testEnumConversionDomains(t, map[string]enumDomainExpectation{
		"Open8Byte":        {"Open8", "uint8", false, IntegerConversionOpenEnumDomain, "0", "255"},
		"Open8Wide":        {"Open8", "uint16", false, IntegerConversionOpenEnumDomain, "0", "255"},
		"Open8SignedWide":  {"Open8", "int16", false, IntegerConversionOpenEnumDomain, "0", "255"},
		"Open8Signed":      {"Open8", "int8", true, "", "", ""},
		"Open16Byte":       {"Open16", "uint8", true, "", "", ""},
		"Open16Wide":       {"Open16", "uint16", false, IntegerConversionOpenEnumDomain, "0", "65535"},
		"Open16Signed":     {"Open16", "int16", true, "", "", ""},
		"Open16SignedWide": {"Open16", "int32", false, IntegerConversionOpenEnumDomain, "0", "65535"},
		"OpenKnownFits":    {"Open16", "uint8", false, IntegerConversionConstant, "1", "1"},
	})
}

type enumDomainExpectation struct {
	source, target   string
	fallible         bool
	basis            IntegerConversionProofBasis
	minimum, maximum string
}

// testEnumConversionDomains checks the public immutable conversion query and
// result type on independently parsed 32/64-bit compilation snapshots.
// Rules: rules/types/types.md — "Exact integer conversion proof";
// rules/compiler/semantic_ir.md — §124.
func testEnumConversionDomains(t *testing.T, expectations map[string]enumDomainExpectation) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/sema/enum_conversion_domains.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			p := parser.New(lexer.New(string(data)))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			seen := map[string]bool{}
			for _, statement := range program.Statements {
				fn, ok := statement.(*ast.FunctionDeclaration)
				if !ok {
					continue
				}
				want, ok := expectations[fn.Name.Value]
				if !ok {
					continue
				}
				_ = walk.Inspect(fn.Body, func(node any) error {
					call, ok := node.(*ast.CallExpression)
					if !ok {
						return nil
					}
					fact, ok := a.ResolvedEnumConversionOf(call)
					if !ok {
						return nil
					}
					seen[fn.Name.Value] = true
					if fact.Kind != ResolvedEnumToInteger || fact.EnumType.Name != want.source || fact.IntegerType.Name != want.target || fact.Fallible != want.fallible || fact.Proof.Basis != want.basis {
						t.Errorf("%s: unexpected fact %s -> %s fallible=%v basis=%q; want %+v", fn.Name.Value, fact.EnumType.Name, fact.IntegerType.Name, fact.Fallible, fact.Proof.Basis, want)
					}
					result, ok := a.ResolvedTypeOf(call)
					if !ok || (want.fallible && (result.Kind != ResultType || result.TypeArgs[0].Name != want.target)) || (!want.fallible && result.Name != want.target) {
						t.Errorf("%s: wrong result type %+v", fn.Name.Value, result)
					}
					if want.basis != "" {
						if fact.Proof.SourceMin == nil || fact.Proof.SourceMax == nil || fact.Proof.SourceMin.String() != want.minimum || fact.Proof.SourceMax.String() != want.maximum {
							t.Errorf("%s: wrong proof domain %+v", fn.Name.Value, fact.Proof)
						}
						if fact.Proof.PlanPointerWidthBits != 0 {
							t.Errorf("%s: fixed-width proof unexpectedly depends on target width: %+v", fn.Name.Value, fact.Proof)
						}
					}
					return nil
				})
			}
			for name := range expectations {
				if !seen[name] {
					t.Errorf("missing conversion fact for %s", name)
				}
			}
		})
	}
}

// TestEnumFixedWidthConversionRejections rejects exact out-of-range members
// and runtime narrowing without try, independently of host/target int width.
// Rules: rules/declarations/enums.md — §12.1;
// rules/types/types.md — "Explicit conversions", "Exact integer conversion proof".
func TestEnumFixedWidthConversionRejections(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_conversion_domains_invalid.sec")
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
		errors := a.Analyze(program)
		if len(errors) != 5 {
			t.Fatalf("%d-bit errors = %v", width, errors)
		}
		for index, want := range []string{"256 overflows uint8", "-1 overflows uint8", "-129 overflows int8", "must return uint8, got Result[uint8,", "must return uint8, got Result[uint8,"} {
			if !strings.Contains(errors[index].Message, want) {
				t.Errorf("%d-bit error %d = %v; want %q", width, index, errors[index], want)
			}
		}
	}
}
