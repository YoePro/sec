package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// @noPanic attaches to the callable body it immediately precedes — init,
// free, each property accessor and a let that directly initializes a lambda —
// and never to the containing impl, property or binding (MD-030).
//
// Rules:
//   - rules/foundations/attributes.md — "Callable-body attachment (MD-030)"
//   - rules/corrections/applied/md030-no-panic-attribute-placement-correction-20261017.md — §§ 2–4, 7, 9(1)
func TestNoPanicAttachesToCallableBodies(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/no_panic_bodies_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	has := func(attributes []*ast.Attribute) bool {
		return len(attributes) == 1 && attributes[0].Name.Value == "noPanic"
	}
	var initDecl *ast.InitDeclaration
	var freeDecl *ast.FreeDeclaration
	properties := map[string]*ast.PropertyDeclaration{}
	lambdas := []*ast.LambdaExpression{}
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.ImplStatement:
			for _, member := range statement.Members {
				switch member := member.(type) {
				case *ast.InitDeclaration:
					initDecl = member
				case *ast.FreeDeclaration:
					freeDecl = member
				case *ast.PropertyDeclaration:
					properties[member.Name.Value] = member
				}
			}
		case *ast.FunctionDeclaration:
			for _, inner := range statement.Body.Statements {
				if let, ok := inner.(*ast.LetStatement); ok {
					if lambda, ok := let.Value.(*ast.LambdaExpression); ok {
						lambdas = append(lambdas, lambda)
						if len(let.Attributes) != 0 {
							t.Errorf("the attribute must not stay on the let binding %s", let.Name.Value)
						}
					}
				}
			}
		}
	}
	if initDecl == nil || !has(initDecl.Attributes) || freeDecl == nil || !has(freeDecl.Attributes) {
		t.Fatalf("init/free attributes: %+v %+v", initDecl, freeDecl)
	}
	value, checked := properties["Value"], properties["Checked"]
	if value == nil || !has(value.GetterAttributes) || !has(value.Setter.Attributes) {
		t.Fatalf("Value accessors: %+v", value)
	}
	if checked == nil || len(checked.GetterAttributes) != 0 || !checked.Setter.Fallible || !has(checked.Setter.Attributes) {
		t.Fatalf("Checked: the getter must not inherit the try set attribute: %+v", checked)
	}
	if len(lambdas) != 3 {
		t.Fatalf("lambdas = %d", len(lambdas))
	}
	for _, lambda := range lambdas {
		if !has(lambda.Attributes) {
			t.Errorf("lambda at %d:%d lacks its body attribute", lambda.Token.Line, lambda.Token.Column)
		}
	}
}

// Shortcuts and indirect forms are rejected with the supported placements:
// @noPanic impl, @noPanic property, a let without a direct lambda and the
// expression-prefix lambda form (MD-030 §§ 5, 8(1), 9(2)).
func TestNoPanicRejectsShortcutsAndIndirectForms(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/no_panic_attachment_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.New(string(source)))
	p.ParseProgram()
	text := strings.Join(p.Errors(), "\n")
	for _, want := range []string{
		"@noPanic cannot annotate a whole property; write it immediately before get, set or try set",
		"@noPanic may only annotate a function, method, extern declaration, init, free, property get/set/try set accessor, or a let that directly initializes a lambda",
		"@noPanic may annotate a local let only when it directly initializes a lambda",
		"@noPanic cannot prefix a lambda expression; write it on its own line before the let",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}
