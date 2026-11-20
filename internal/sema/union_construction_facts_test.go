package sema

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestUnionPreparationFacts checks stored concrete constructor and operand
// facts across all construction spellings, defaults and ownership commits.
// Rules: rules/declarations/unions.md §§4–5,7.8;
// rules/compiler/semantic_ir.md §§23,35–36.
func TestUnionPreparationFacts(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/union_preparation_facts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("%d/%s", width, depth), func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(source), "union-preparation.sec"))
				parsed := p.Parse()
				if parsed.HasErrors {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				assertSemaErrors(t, a.Analyze(parsed.Program), nil)
				functions := map[string]*ast.FunctionDeclaration{}
				for _, statement := range parsed.Program.Statements {
					if f, ok := statement.(*ast.FunctionDeclaration); ok {
						functions[f.Name.Value] = f
					}
				}
				plans := map[string]ResolvedUnionConstruction{}
				for _, name := range []string{"Empty", "EmptyContext", "Copy", "GenericContext", "ExplicitMove", "TerminalMove", "Temporary", "Reordered", "Defaults", "DefaultsInt", "Float"} {
					body := functions[name].Body.Statements
					expr := body[len(body)-1].(*ast.ReturnStatement).Value
					plan, ok := a.ResolvedUnionConstructionOf(expr)
					if !ok {
						t.Fatalf("%s has no constructor fact", name)
					}
					plans[name] = plan
					if !reflect.DeepEqual(plan, a.resolvedUnionConstructions[expr]) {
						t.Fatalf("%s query changed stored facts", name)
					}
				}
				for _, name := range []string{"Empty", "EmptyContext"} {
					if plans[name].Kind != ResolvedUnionVariantEmpty || len(plans[name].PayloadPreparation) != 0 {
						t.Fatal(name, plans[name])
					}
				}
				for _, name := range []string{"ExplicitMove", "TerminalMove"} {
					operand := plans[name].PayloadPreparation[0]
					if operand.Action != StructFieldMove || operand.Transfer == nil || operand.Transfer.Action != TransferMove || operand.Transfer.Source.Root != "value" {
						t.Fatalf("%s: %+v", name, operand)
					}
				}
				if plans["Temporary"].PayloadPreparation[0].Action != StructFieldConstructDirect {
					t.Fatal(plans["Temporary"])
				}
				copy := plans["Copy"].PayloadPreparation[0]
				if copy.Action != StructFieldCopyTrivial || copy.Transfer == nil || copy.Transfer.Action != TransferCopyTrivial || copy.Type.Kind != ArrayType || copy.Type.Element.Name != "int" {
					t.Fatal(copy)
				}
				if plans["GenericContext"].PayloadPreparation[0].Type.MaxInteger == nil || plans["GenericContext"].PayloadPreparation[0].Type.MaxInteger.String() != "255" || plans["Float"].PayloadPreparation[0].Type.FloatBits != 32 {
					t.Fatalf("exact substituted scalar type lost: uint=%+v float=%+v", plans["GenericContext"].PayloadPreparation[0].Type, plans["Float"].PayloadPreparation[0].Type)
				}
				order := plans["Reordered"]
				if !reflect.DeepEqual(order.CanonicalFields, []string{"count", "owner"}) || !reflect.DeepEqual(order.SourceFieldOrder, []string{"owner", "count"}) || order.PayloadPreparation[0].FieldIndex != 1 || order.PayloadPreparation[1].FieldIndex != 0 {
					t.Fatal(order)
				}
				defaults := plans["Defaults"].PayloadPreparation
				if len(defaults) != 2 || defaults[0].Defaulted || !defaults[1].Defaulted || defaults[1].FieldName != "right" || defaults[1].Type.FloatBits != 32 || defaults[1].Default == nil || defaults[1].Default.Kind != PrimitiveDefault {
					t.Fatal(defaults)
				}
				// Reusing the same AST must preserve the origin of materialized defaults.
				assertSemaErrors(t, a.Analyze(parsed.Program), nil)
				repeatedDefault, _ := a.ResolvedUnionConstructionOf(functions["Defaults"].Body.Statements[0].(*ast.ReturnStatement).Value)
				if !repeatedDefault.PayloadPreparation[1].Defaulted {
					t.Fatal("synthesized default became source syntax on reanalysis")
				}
				defaultExpr := functions["Defaults"].Body.Statements[0].(*ast.ReturnStatement).Value
				beforeDefault, _ := a.ResolvedUnionConstructionOf(defaultExpr)
				mutatedDefault, _ := a.ResolvedUnionConstructionOf(defaultExpr)
				mutatedDefault.PayloadPreparation[0].Type.MaxInteger.SetInt64(999)
				afterDefault, _ := a.ResolvedUnionConstructionOf(defaultExpr)
				if !reflect.DeepEqual(beforeDefault, afterDefault) {
					t.Fatal("query shared exact scalar/transfer domains")
				}
				intDefaultExpr := functions["DefaultsInt"].Body.Statements[0].(*ast.ReturnStatement).Value
				intDefaultBefore, _ := a.ResolvedUnionConstructionOf(intDefaultExpr)
				intDefaultMutated, _ := a.ResolvedUnionConstructionOf(intDefaultExpr)
				intDefaultMutated.PayloadPreparation[1].Default.Value.Integer.SetInt64(999)
				intDefaultAfter, _ := a.ResolvedUnionConstructionOf(intDefaultExpr)
				if !reflect.DeepEqual(intDefaultBefore, intDefaultAfter) {
					t.Fatal("query shared exact default values")
				}
				ordinary := functions["Ordinary"].Body.Statements[0].(*ast.LetStatement).Value
				if _, ok := a.ResolvedUnionConstructionOf(ordinary); ok {
					t.Fatal("ordinary function named like a variant became a constructor")
				}
				// Saved metadata cannot be changed by AST spelling mutations or
				// by a caller changing nested fact/Place/type data.
				expr := functions["ExplicitMove"].Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.CallExpression)
				before, _ := a.ResolvedUnionConstructionOf(expr)
				changed := plans["ExplicitMove"]
				changed.UnionType.UnionVariants[1].Payload.Fields[0].Name = "changed"
				changed.PayloadActions[0] = StructFieldCopyTrivial
				changed.PayloadPreparation[0].Type.Fields[0].Name = "changed"
				changed.PayloadPreparation[0].Transfer.Source.Type.Fields[0].Name = "changed"
				changed.PayloadPreparation[0].Transfer.Source.Root = "changed"
				expr.Callee.(*ast.MemberExpression).Property.Value = "Empty"
				after, _ := a.ResolvedUnionConstructionOf(expr)
				if !reflect.DeepEqual(before, after) {
					t.Fatal("query reconstructed AST spelling or exposed mutable storage")
				}
				// New analysis replaces the registry, so no former AST survives.
				fresh := parser.New(lexer.New("module main\nfn main() void {}")).Parse()
				assertSemaErrors(t, a.Analyze(fresh.Program), nil)
				if _, ok := a.ResolvedUnionConstructionOf(expr); ok {
					t.Fatal("stale constructor survived reanalysis")
				}
			})
		}
	}
}

// TestInvalidUnionPreparationFacts rejects publication after invalid payload
// typing, representability, arity, field or ownership checks.
// Rules: rules/declarations/unions.md §§5,7.8;
// rules/compiler/compiler_analysis.md §55 "Analysis snapshot consistency".
func TestInvalidUnionPreparationFacts(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/union_preparation_facts_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "union-preparation-invalid.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(parsed.Program); len(errors) < 8 {
		t.Fatalf("missing invalid cases: %+v", errors)
	}
	if err := walk.Inspect(parsed.Program, func(node any) error {
		if expr, ok := node.(ast.Expression); ok {
			if plan, found := a.ResolvedUnionConstructionOf(expr); found {
				t.Fatalf("invalid constructor published %+v", plan)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var absent *Analyzer
	if _, ok := absent.ResolvedUnionConstructionOf(&ast.CallExpression{}); ok {
		t.Fatal("nil analyzer had facts")
	}
	if _, ok := a.ResolvedUnionConstructionOf(nil); ok {
		t.Fatal("nil expression had facts")
	}
}
