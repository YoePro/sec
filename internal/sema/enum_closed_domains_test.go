package sema

import (
	"math/big"
	"os"
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// Wide carriers do not enlarge closed enum domains; repeated aliases do not
// enlarge unique numeric classes, including negative-only and sparse domains.
// Rules: rules/declarations/enums.md — §§7,9,12.1;
// rules/types/types.md — "Exact integer conversion proof" (MD-104).
func TestClosedEnumConversionUsesUniqueClasses(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_closed_domains.sec")
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
		seen := 0
		_ = walk.Inspect(program, func(node any) error {
			call, ok := node.(*ast.CallExpression)
			if !ok {
				return nil
			}
			fact, ok := a.ResolvedEnumConversionOf(call)
			if !ok {
				return nil
			}
			seen++
			if fact.EnumType.Name == "Beyond" {
				if !fact.Fallible || fact.Proof.Proven() {
					t.Fatal(fact)
				}
				return nil
			}
			bounds := map[string][2]string{"Sparse": {"-128", "127"}, "Negative": {"-128", "-1"}, "Byte": {"0", "255"}}[fact.EnumType.Name]
			if fact.Fallible || fact.Proof.Basis != IntegerConversionClosedEnumDomain || fact.Proof.SourceMin.String() != bounds[0] || fact.Proof.SourceMax.String() != bounds[1] || len(EnumValueClasses(fact.EnumType)) != 2 {
				t.Fatalf("%d-bit proof: %+v", width, fact)
			}
			return nil
		})
		if seen != 4 {
			t.Fatalf("conversions: %d", seen)
		}
	}
}

// A closed-domain proof requires a complete, consistent declaration inventory;
// recovery/stale metadata cannot silently omit an unrepresentable member.
// Rules: rules/types/types.md — "Exact integer conversion proof";
// rules/compiler/semantic_ir.md — §124 (fail closed on missing evidence).
func TestClosedEnumConversionRejectsIncompleteDomain(t *testing.T) {
	a := NewAnalyzer()
	target := a.Types()["uint8"]
	for name, source := range map[string]Type{
		"no declaration order":        {Kind: EnumType, EnumConsts: map[string]EnumValue{"Small": {Name: "Small", Value: big.NewInt(1)}}},
		"missing declared member":     {Kind: EnumType, EnumValues: []string{"Small", "Large"}, EnumConsts: map[string]EnumValue{"Small": {Name: "Small", Value: big.NewInt(1)}}},
		"undeclared numeric entry":    {Kind: EnumType, EnumValues: []string{"Small"}, EnumConsts: map[string]EnumValue{"Small": {Name: "Small", Value: big.NewInt(1)}, "Large": {Name: "Large", Value: big.NewInt(300)}}},
		"duplicate declaration entry": {Kind: EnumType, EnumValues: []string{"Small", "Small"}, EnumConsts: map[string]EnumValue{"Small": {Name: "Small", Value: big.NewInt(1)}, "Large": {Name: "Large", Value: big.NewInt(300)}}},
		"missing numeric value":       {Kind: EnumType, EnumValues: []string{"Small", "Large"}, EnumConsts: map[string]EnumValue{"Small": {Name: "Small", Value: big.NewInt(1)}, "Large": {Name: "Large"}}},
	} {
		t.Run(name, func(t *testing.T) {
			if proof := a.integerConversionProof(target, source, nil); proof.Proven() {
				t.Fatalf("invented complete domain: %+v", proof)
			}
		})
	}
}
