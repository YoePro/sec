package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestConstructionSignaturesAreReadOnlyPreviews verifies declarations remain
// available when overload selection fails, without fabricating a selected init
// or mutating analyzer state when tooling inspects candidate metadata.
// Rules: rules/declarations/impl.md — §§15.4,15.6;
// rules/tooling/lsp.md — §§4.1(22),13, "Snapshots".
func TestConstructionSignaturesAreReadOnlyPreviews(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/initializer_signatures.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Replace(string(data), "new Widget(1)", "new Widget(true)", 1)
	parsed := parser.New(lexer.New(source))
	program := parsed.ParseProgram()
	if len(parsed.Errors()) != 0 {
		t.Fatal(parsed.Errors())
	}
	a := NewAnalyzer()
	if len(a.Analyze(program)) == 0 {
		t.Fatal("invalid construction unexpectedly accepted")
	}
	var construction *ast.NewExpression
	for expression := range a.constructionSignatures {
		construction = expression
	}
	if construction == nil {
		t.Fatal("failed overload selection lost declaration previews")
	}
	if _, selected := a.ResolvedConstructionOf(construction); selected {
		t.Fatal("candidate previews fabricated a selected construction")
	}
	before := a.ConstructionSignaturesOf(construction)
	if len(before) != 4 || before[0].Parameters[0].Name != "value" || !before[3].Implicit || before[2].ErrorType == nil {
		t.Fatalf("candidate metadata = %+v", before)
	}
	mutated := a.ConstructionSignaturesOf(construction)
	mutated[0].Parameters[0].Name = "changed"
	mutated[1].Parameters[0].Type.Element.Name = "changed"
	mutated[2].ErrorType.Name = "changed"
	for _, value := range mutated[2].ErrorType.EnumConsts {
		if value.Value != nil {
			value.Value.SetInt64(999)
		}
	}
	mutated[0].Target.Fields[0].Type.Name = "changed"
	if after := a.ConstructionSignaturesOf(construction); !reflect.DeepEqual(before, after) {
		t.Fatal("tooling mutation changed candidate facts")
	}
	if after := a.ConstructionSignaturesOf(&ast.NewExpression{}); len(after) != 0 {
		t.Fatal("unknown expression acquired candidates")
	}
	if after := (*Analyzer)(nil).ConstructionSignaturesOf(construction); len(after) != 0 {
		t.Fatal("nil analyzer acquired candidates")
	}
	// A fresh analysis cannot serve stale previews keyed by the old AST.
	a.Analyze(program)
	if after := a.ConstructionSignaturesOf(construction); !reflect.DeepEqual(before, after) {
		t.Fatal("equivalent reanalysis changed candidate ordering or contracts")
	}
	empty := parser.New(lexer.New("module sample\nfn Empty() void {}\n")).ParseProgram()
	a.Analyze(empty)
	if after := a.ConstructionSignaturesOf(construction); len(after) != 0 {
		t.Fatal("new analysis retained old candidate metadata")
	}
}
