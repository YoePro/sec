package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestStructSpreadModuleConstructionBoundaries checks that a same-type source
// cannot expose hidden representation, suppress required inaccessible fields,
// or waive field contracts. Valid public/local updates retain construction plans.
// Rules: rules/declarations/spread.md — §§6.1,6.3,6.4;
// rules/declarations/struct.md — §§5,9;
// rules/foundations/names_scopes_visibility.md — §12.2.
func TestStructSpreadModuleConstructionBoundaries(t *testing.T) {
	program := &ast.Program{}
	for _, name := range []string{"owner.sec", "consumer.sec"} {
		path := "../../testdata/sema/spread_visibility/" + name
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), name))
		part := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.Statements = append(program.Statements, part.Statements...)
	}
	a := NewAnalyzer()
	errors := a.Analyze(program)
	for _, err := range errors {
		if err.File != "consumer.sec" {
			t.Errorf("valid construction in the owning module was rejected: %v", err)
		}
	}
	for _, want := range []string{"field _representation on Hidden is not accessible from module client", "field _reference on HiddenRef is not accessible from module client", "field \"_reference\" in struct HiddenRef has no default value", "cannot spread Other into Public", "range"} {
		found := false
		for _, err := range errors {
			found = found || strings.Contains(err.Message, want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, errors)
		}
	}
	valid := map[string]bool{"CopyInside": true, "CopyRefInside": true, "CopyPublic": true}
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		_ = walk.Inspect(fn.Body, func(node any) error {
			literal, ok := node.(*ast.StructLiteral)
			if !ok {
				return nil
			}
			_, hasPlan := a.ResolvedStructLiteralPlanOf(literal)
			if hasPlan != valid[fn.Name.Value] {
				t.Errorf("%s: valid construction plan=%v; want %v", fn.Name.Value, hasPlan, valid[fn.Name.Value])
			}
			return nil
		})
	}
}

// TestSpreadOutOfContextIsRejected confirms that retaining a complete postfix
// AST for recovery never makes standalone expansion a legal value.
// Rules: rules/declarations/spread.md — §§1–2.
func TestSpreadOutOfContextIsRejected(t *testing.T) {
	path := "../../testdata/parser/spread_out_of_context_invalid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errors := NewAnalyzer().Analyze(program)
	count := 0
	for _, err := range errors {
		if strings.Contains(err.Message, "spread operator is not valid as a standalone expression") {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("both standalone spreads must be rejected: %v", errors)
	}
}
