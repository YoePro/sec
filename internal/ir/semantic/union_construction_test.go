package semantic

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestUnionConstructionConsumesPreparation proves Sema's source order and
// operand identity remain authoritative when constructor AST shape changes.
// Rules: rules/declarations/unions.md §§5,7.8;
// rules/compiler/semantic_ir.md §§23,35–36.
func TestUnionConstructionConsumesPreparation(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/union_preparation_consumers.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(data), "union-preparation.sec"))
			parsed := p.Parse()
			if parsed.HasErrors {
				t.Fatal(p.Errors())
			}
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := a.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			options := BuildOptions{RequestedModule: "main", SourceFiles: []string{"union-preparation.sec"}, MaxPackage: 17}
			before, err := Build(parsed.Program, a, options)
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(before); err != nil {
				t.Fatal(err)
			}
			var ordered []string
			for _, fn := range before.Functions {
				if fn.Name != "Ordered" {
					continue
				}
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						if op.Kind == OpDirectCall {
							ordered = append(ordered, string(op.Callee))
						}
						if op.Kind == OpUnionConstruct && (!reflect.DeepEqual(op.UnionFields, []string{"left", "right"}) || !reflect.DeepEqual(op.PayloadActions, []UnionPayloadAction{UnionPayloadCopyTrivial, UnionPayloadCopyTrivial})) {
							t.Fatalf("canonical assembly: %+v", op)
						}
					}
				}
			}
			if len(ordered) != 2 || !strings.Contains(ordered[0], "Second(") || !strings.Contains(ordered[1], "First(") {
				t.Fatal("source evaluation order:", ordered)
			}
			// The compiler owns AST nodes, but this deliberate corruption
			// proves the consumer is using saved operands, not reconstructing them.
			err = walk.Inspect(parsed.Program, func(node any) error {
				expr, ok := node.(ast.Expression)
				if !ok {
					return nil
				}
				if _, known := a.ResolvedUnionConstructionOf(expr); !known {
					return nil
				}
				switch value := expr.(type) {
				case *ast.StructLiteral:
					value.Fields = nil
				case *ast.CallExpression:
					value.Arguments = nil
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			after, err := Build(parsed.Program, a, options)
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(after); err != nil {
				t.Fatal(err)
			}
			if Format(before) != Format(after) {
				t.Fatal("construction output depended on constructor AST operands")
			}
		})
	}
}

// TestUnionConstructionRejectsInvalidPreparation ensures unavailable or
// inconsistent plans cannot trigger operand evaluation or a syntax fallback.
// Rules: rules/compiler/semantic_ir.md §§2,23,35–36;
// rules/compiler/compiler_pipeline.md §32.
func TestUnionConstructionRejectsInvalidPreparation(t *testing.T) {
	source := `module main
type U union { Pair { left: int, right: int, } }
fn Make() U { return U.Pair { right: 2, left: 1 } }
`
	parsed := parser.New(lexer.New(source)).Parse()
	a := sema.NewAnalyzer()
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	module, err := Build(parsed.Program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	expr := parsed.Program.Statements[2].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value
	b := &builder{module: module, analyzer: a, maxPackage: 17, definedEnums: map[TypeID]bool{}, definedUnions: map[TypeID]bool{}, definedStructs: map[TypeID]bool{}}
	for _, u := range module.Unions {
		b.definedUnions[u.TypeID] = true
	}
	base, _ := a.ResolvedUnionConstructionOf(expr)
	resultType, err := b.internType(base.UnionType)
	if err != nil {
		t.Fatal(err)
	}
	fb := &functionBuilder{owner: b}
	if _, err := fb.validateUnionPreparation(expr, base, resultType); err != nil {
		t.Fatal(err)
	}
	// Legacy slices are compatibility views; they cannot override operand facts.
	base.PayloadActions = nil
	base.SourceFieldOrder = nil
	base.CanonicalFields = nil
	if _, err := fb.validateUnionPreparation(expr, base, resultType); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*sema.ResolvedUnionConstruction)
	}{
		{"missing", func(p *sema.ResolvedUnionConstruction) { p.PayloadPreparation = nil }},
		{"nil source", func(p *sema.ResolvedUnionConstruction) { p.PayloadPreparation[0].Source = nil }},
		{"duplicate destination", func(p *sema.ResolvedUnionConstruction) {
			p.PayloadPreparation[0].FieldIndex = p.PayloadPreparation[1].FieldIndex
		}},
		{"negative index", func(p *sema.ResolvedUnionConstruction) { p.PayloadPreparation[0].FieldIndex = -1 }},
		{"out of range", func(p *sema.ResolvedUnionConstruction) { p.PayloadPreparation[0].FieldIndex = 10 }},
		{"wrong name", func(p *sema.ResolvedUnionConstruction) { p.PayloadPreparation[0].FieldName = "left" }},
		{"wrong type", func(p *sema.ResolvedUnionConstruction) {
			p.PayloadPreparation[0].Type = sema.Type{Name: "bool", Kind: sema.BoolType}
		}},
		{"missing default", func(p *sema.ResolvedUnionConstruction) { p.PayloadPreparation[0].Defaulted = true }},
		{"unexpected default", func(p *sema.ResolvedUnionConstruction) {
			p.PayloadPreparation[0].Default = &sema.DefaultResolution{Kind: sema.PrimitiveDefault}
		}},
		{"wrong union type", func(p *sema.ResolvedUnionConstruction) { p.UnionType = sema.Type{Name: "bool", Kind: sema.BoolType} }},
		{"out of range variant", func(p *sema.ResolvedUnionConstruction) { p.VariantIndex = ^uint32(0) }},
		{"wrong variant", func(p *sema.ResolvedUnionConstruction) { p.VariantName = "Other" }},
		{"wrong kind", func(p *sema.ResolvedUnionConstruction) { p.Kind = sema.ResolvedUnionVariantSingle }},
		{"semantic copy", func(p *sema.ResolvedUnionConstruction) {
			p.PayloadPreparation[0].Action = sema.StructFieldCopySemanticInfallible
		}},
		{"unknown action", func(p *sema.ResolvedUnionConstruction) { p.PayloadPreparation[0].Action = "guessed" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, _ := a.ResolvedUnionConstructionOf(expr)
			test.mutate(&plan)
			_, err := fb.buildUnionConstruction(expr, plan, resultType)
			var unsupported *UnsupportedFeatureError
			if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), "union payload preparation") {
				t.Fatalf("%v", err)
			}
		})
	}
}
