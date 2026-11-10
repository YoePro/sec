package semantic

import (
	"os"
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"testing"
)

// TestOverloadNestedNominalIdentity ensures references and sequences retain
// their element's declaring module, rather than using an ambiguous display name.
// Rules: rules/declarations/functions.md — §§19–20;
// rules/types/types.md — Type identity; rules/compiler/semantic_ir.md — §61.
func TestOverloadNestedNominalIdentity(t *testing.T) {
	left := sema.Type{Name: "Value", Module: "left", Kind: sema.IntType, Named: true, Declared: true}
	right := left
	right.Module = "right"
	for _, kind := range []sema.TypeKind{sema.ReferenceType, sema.ArrayType, sema.SliceType} {
		a := sema.Type{Name: "same display spelling", Kind: kind, Element: &left, ArrayShape: sema.ArrayShapeDynamic}
		b := a
		b.Element = &right
		fa := sema.Function{Name: "Select", Module: "main", Parameters: []sema.FunctionParameter{{Type: a}}}
		fb := fa
		fb.Parameters = []sema.FunctionParameter{{Type: b}}
		if functionID(fa, nil) == functionID(fb, nil) {
			t.Fatalf("%v lost nominal element module", kind)
		}
	}
}

// TestOverloadSelectedCallIdentity compares each direct IR call against its
// exact source-position Sema selection, including nested nominal overloads.
// Rules: rules/compiler/semantic_ir.md — §61(1–4);
// rules/declarations/functions.md — §§11,19–20.
func TestOverloadSelectedCallIdentity(t *testing.T) {
	const file = "../../../testdata/build/native/overload_nominal_identity.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
		if err != nil {
			t.Fatal(err)
		}
		selected := map[Location]FunctionID{}
		if err := walk.Inspect(program, func(node any) error {
			if call, ok := node.(*ast.CallExpression); ok {
				if target, found := a.ResolvedCallTarget(call); found {
					selected[location(call.Token)] = functionID(target.Function, module.Types)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, fn := range module.Functions {
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					if op.Kind != OpDirectCall {
						continue
					}
					if expected := selected[op.Location]; expected == "" || op.Callee != expected {
						t.Fatalf("call at %+v: got %s, selected %s", op.Location, op.Callee, expected)
					}
					count++
				}
			}
		}
		if count != 5 || len(selected) != 5 {
			t.Fatalf("calls %d selections %d, want five", count, len(selected))
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
	}
}
