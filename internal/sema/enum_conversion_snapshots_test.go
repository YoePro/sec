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

// Conversion queries return detached evidence rather than writable references
// to the enum declaration or a new proof computed under changed analyzer state.
// Rules: rules/types/types.md — Exact integer conversion proof (MD-104);
// rules/compiler/semantic_ir.md — §124; compiler_analysis.md — immutable results.
func TestEnumConversionFactsAreDetached(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_conversion_snapshots.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	seen := 0
	_ = walk.Inspect(program, func(node any) error {
		call, ok := node.(*ast.CallExpression)
		if !ok {
			return nil
		}
		before, ok := a.ResolvedEnumConversionOf(call)
		if !ok {
			return nil
		}
		seen++
		sourceName, targetName, fallible, basis := before.EnumType.Name, before.IntegerType.Name, before.Fallible, before.Proof.Basis
		for key, value := range before.EnumType.EnumConsts {
			if value.Value != nil {
				value.Value.SetInt64(999)
			}
			delete(before.EnumType.EnumConsts, key)
		}
		before.EnumType.EnumValues[0] = "changed"
		fixed := len(before.EnumType.TypeArgs) > 0 && before.EnumType.TypeArgs[0].FixedNamedArguments
		if len(before.EnumType.TypeArgs) > 0 {
			before.EnumType.TypeArgs[0].Name = "changed"
			for name, value := range before.EnumType.TypeArgs[0].EnumConsts {
				if value.Value != nil {
					value.Value.SetInt64(999)
				}
				if value.StringValue != nil {
					*value.StringValue = "changed"
				}
				delete(before.EnumType.TypeArgs[0].EnumConsts, name)
			}
		}
		if before.IntegerType.MinInteger != nil {
			before.IntegerType.MinInteger.SetInt64(999)
		}
		if before.Proof.SourceMin != nil {
			before.Proof.SourceMin.SetInt64(999)
		}
		if before.Proof.TargetMax != nil {
			before.Proof.TargetMax.SetInt64(999)
		}
		after, ok := a.ResolvedEnumConversionOf(call)
		if !ok || after.EnumType.Name != sourceName || after.IntegerType.Name != targetName || after.Fallible != fallible || after.Proof.Basis != basis || len(after.EnumType.EnumConsts) == 0 || after.EnumType.EnumValues[0] == "changed" {
			t.Fatalf("query mutation escaped: %+v", after)
		}
		for _, value := range after.EnumType.EnumConsts {
			if value.Value == nil || value.Value.Cmp(big.NewInt(999)) == 0 {
				t.Fatalf("enum value leaked: %+v", value)
			}
		}
		if len(after.EnumType.TypeArgs) > 0 {
			argument := after.EnumType.TypeArgs[0]
			if argument.Name == "changed" || argument.FixedNamedArguments != fixed {
				t.Fatal("generic identity leaked")
			}
			if argument.Kind == EnumType {
				if len(argument.EnumConsts) == 0 {
					t.Fatal("nested enum inventory leaked")
				}
				if argument.Name == "Wide" && argument.EnumConsts["Small"].Value.Int64() != 1 {
					t.Fatal("numeric argument leaked")
				}
				if argument.Name == "Words" && *argument.EnumConsts["One"].StringValue != "one" {
					t.Fatal("string argument leaked")
				}
			}
		}
		if after.IntegerType.MinInteger != nil && after.IntegerType.MinInteger.Int64() == 999 {
			t.Fatal("target bounds leaked")
		}
		if after.Proof.SourceMin != nil && after.Proof.SourceMin.Int64() == 999 {
			t.Fatal("proof bounds leaked")
		}
		return nil
	})
	if seen != 8 {
		t.Fatalf("conversion facts: %d", seen)
	}
}

// Querying retained facts does not re-run analysis under a different plan.
// A fresh analysis resets those facts and computes the new plan's fallibility.
// Rules: rules/types/types.md — Exact integer conversion proof (MD-104 Option A);
// rules/compiler/semantic_ir.md — §124.
func TestEnumConversionDecisionIsRecordedPerAnalysis(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/enum_conversion_snapshots.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	fn := program.Statements[len(program.Statements)-1].(*ast.FunctionDeclaration)
	call := fn.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.CallExpression)
	a.scalarPlanWidthBits = 32
	old, ok := a.ResolvedEnumConversionOf(call)
	if !ok || old.Fallible || old.Proof.PlanPointerWidthBits != 64 || !old.Proof.Proven() {
		t.Fatalf("query recomputed decision: %+v", old)
	}
	errors := a.Analyze(program)
	if len(errors) != 1 {
		t.Fatalf("fresh 32-bit analysis: %v", errors)
	}
	fresh, ok := a.ResolvedEnumConversionOf(call)
	if !ok || !fresh.Fallible || fresh.Proof.Proven() {
		t.Fatalf("stale plan reused: %+v", fresh)
	}
	a.resolvedEnumConversions = nil
	if _, found := a.ResolvedEnumConversionOf(call); found {
		t.Fatal("query manufactured missing producer facts")
	}

}
