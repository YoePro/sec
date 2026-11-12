package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// A @target written as the file's first source-bearing line selects the
// whole file and becomes the file's canonical target selection; later
// @target and @when attach to the next top-level statement of any kind,
// including imports and impl blocks, and never to the module declaration.
// Rules: rules/foundations/attributes.md — §§ 15, 16, 17, 26.
func TestCanonicalTargetForms(t *testing.T) {
	source := "// leading comment\n@target(os: \"linux\", device: \"controller-a\")\n\nmodule main\n\n@target(os: \"linux\")\nimport \"fmt\"\n\ntype A struct {\n    v: int,\n}\n\n@when(config.audit)\nimpl A {\n    fn Get() int {\n        return self.v\n    }\n}\n"
	p := New(lexer.NewWithFile(source, "main.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	var directive *ast.TargetDirective
	var imported *ast.ImportStatement
	var impl *ast.ImplStatement
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.TargetDirective:
			directive = statement
		case *ast.ImportStatement:
			imported = statement
		case *ast.ImplStatement:
			impl = statement
		}
	}
	if directive == nil || !directive.Canonical || directive.OS != "linux" || len(directive.Selectors) != 2 || directive.Selectors[1].Name != "device" || directive.Selectors[1].Value != "controller-a" {
		t.Fatalf("file-level target = %+v", directive)
	}
	if imported == nil || len(ast.StatementAttributes(imported)) != 1 || impl == nil || len(ast.StatementAttributes(impl)) != 1 || ast.StatementAttributes(impl)[0].Name.Value != "when" {
		t.Errorf("statement-level attributes: import %+v impl %+v", imported, impl)
	}

	for name, text := range map[string]string{
		"accompanied": "@target(os: \"linux\")\n@noAlloc\nfn Run() void {\n}\n",
		"module":      "fn Run() void {\n}\n\n@target(os: \"linux\")\nmodule main\n",
	} {
		p := New(lexer.NewWithFile(text, "main.sec"))
		p.ParseProgram()
		joined := strings.Join(p.Errors(), "\n")
		want := map[string]string{"accompanied": "cannot accompany a file-level @target", "module": "does not attach to a module declaration"}[name]
		if !strings.Contains(joined, want) {
			t.Errorf("%s: errors = %v", name, p.Errors())
		}
	}
}
